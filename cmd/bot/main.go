// Package main is the entry point for the Throttle Bot Telegram bot application.
// It initializes the bot, connects to the database, and starts the message handler.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/antst/tg-throttle-bot/internal/bot"
	"github.com/antst/tg-throttle-bot/internal/config"
	"github.com/antst/tg-throttle-bot/internal/i18n"
	"github.com/antst/tg-throttle-bot/internal/logging"
	"github.com/antst/tg-throttle-bot/internal/ratelimit"
	"github.com/antst/tg-throttle-bot/internal/storage"
	"github.com/antst/tg-throttle-bot/internal/sync"
	"github.com/antst/tg-throttle-bot/internal/telegram"
	"github.com/antst/tg-throttle-bot/migrations"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize structured logger
	logger, err := logging.NewLogger(cfg.LogLevel)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() {
		if err := logger.Close(); err != nil {
			log.Printf("Error closing logger: %v", err)
		}
	}()

	logger.Infow(
		"Starting Throttle Bot",
		"log_level", cfg.LogLevel,
	)

	// Initialize i18n bundle
	if err := i18n.Init(); err != nil {
		logger.Fatalw(
			"Failed to initialize i18n",
			"error", err,
		)
	}
	logger.Info("i18n initialized successfully with embedded translations")

	// Initialize database
	ctx := context.Background()
	pgStore, err := storage.NewPostgresStore(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatalw(
			"Failed to connect to database",
			"error", err,
			"database_url", maskDatabaseURL(cfg.DatabaseURL),
		)
	}

	// Run database migrations
	if err := runMigrations(cfg.DatabaseURL, logger); err != nil {
		logger.Warnw(
			"Migration failed - continuing anyway",
			"error", err,
		)
	}

	// Create rate limit storage adapter
	rateLimitStore := storage.NewRateLimitStorage(pgStore)

	// Create Telegram client
	client, err := telegram.NewClient(cfg.TelegramToken, cfg.LogLevel == "debug")
	if err != nil {
		pgStore.Close()
		logger.Fatalw(
			"Failed to create Telegram client",
			"error", err,
		)
	}

	// Get bot user ID for permission checking
	botUser, err := client.GetBotAPI().GetMe()
	if err != nil {
		pgStore.Close()
		logger.Fatalw(
			"Failed to get bot info",
			"error", err,
		)
	}
	botUserID := botUser.ID

	logger.Infow(
		"Bot authenticated successfully",
		"username", botUser.UserName,
		"bot_id", botUserID,
	)

	// Create message handler with logger
	handler := bot.NewHandlerWithLogger(rateLimitStore, client, logger)

	// Create sync coordinator for proactive member synchronization (Feature 011)
	syncCoordinator := sync.NewSyncCoordinator(client, rateLimitStore, logger.Desugar())
	handler.SetSyncCoordinator(syncCoordinator)
	logger.Info("Sync coordinator initialized for proactive member synchronization")

	// Perform startup synchronization for all existing groups (Feature 013 enhancement)
	logger.Info("Performing startup member synchronization for existing groups...")
	if err := syncCoordinator.StartupSync(ctx); err != nil {
		logger.Warnw("Startup synchronization completed with errors", "error", err)
	} else {
		logger.Info("Startup synchronization completed successfully")
	}

	// Create and start background worker for cleanup
	worker := ratelimit.NewWorker(rateLimitStore, client.GetBotAPI())
	go worker.Start(ctx)

	logger.Info("Background worker started (cleanup every 5min: old messages + expired overrides)")

	// Create and start periodic sync worker (Feature 011: 24h member sync)
	syncWorker := sync.NewPeriodicSyncWorker(syncCoordinator, rateLimitStore, logger.Desugar(), 24*time.Hour)
	go syncWorker.Start(ctx)

	logger.Info("Periodic sync worker started (every 24h: group member synchronization)")

	// Get MessageRouter for permission notifications (Feature 009 US4: MessageRouter compliance)
	router := handler.GetRouter()

	// Setup permission loss/restore callbacks
	onPermissionLost := func(chatID int64, missing []string) {
		logger.Warnw(
			"Permissions lost in chat - disabling rate limiting",
			"chat_id", chatID,
			"missing_permissions", missing,
		)
		// Disable rate limiting for this chat
		if err := rateLimitStore.SetGroupPaused(ctx, chatID, true, nil); err != nil {
			logger.Errorw(
				"Failed to disable rate limiting after permission loss",
				"chat_id", chatID,
				"error", err,
			)
		}

		// Send notification to chat admins (FR-022, Feature 009 US4: localized)
		// Note: Using group's default language since this is a system notification
		groupLang, err := rateLimitStore.GetGroupLanguage(ctx, chatID)
		if err != nil || groupLang == "" {
			groupLang = "en"
		}

		permissionsStr := strings.Join(missing, ", ")
		localizer := i18n.GetLocalizer(groupLang)
		message := i18n.Localize(localizer, "permission_lost", map[string]interface{}{
			"permissions": permissionsStr,
		})

		if err := router.SendGroupNotification(ctx, chatID, message); err != nil {
			logger.Errorw(
				"Failed to send permission lost notification",
				"chat_id", chatID,
				"error", err,
			)
		}
	}

	onPermissionRestored := func(chatID int64) {
		logger.Infow(
			"Permissions restored in chat - re-enabling rate limiting",
			"chat_id", chatID,
		)
		// Re-enable rate limiting for this chat (FR-023)
		if err := rateLimitStore.SetGroupPaused(ctx, chatID, false, nil); err != nil {
			logger.Errorw(
				"Failed to re-enable rate limiting after permission restoration",
				"chat_id", chatID,
				"error", err,
			)
		}

		// Send notification to chat admins (Feature 009 US4: localized)
		// Note: Using group's default language since this is a system notification
		groupLang, err := rateLimitStore.GetGroupLanguage(ctx, chatID)
		if err != nil || groupLang == "" {
			groupLang = "en"
		}

		localizer := i18n.GetLocalizer(groupLang)
		message := i18n.Localize(localizer, "permission_restored", map[string]interface{}{})

		if err := router.SendGroupNotification(ctx, chatID, message); err != nil {
			logger.Errorw(
				"Failed to send permission restored notification",
				"chat_id", chatID,
				"error", err,
			)
		}
	}

	// Get all configured group IDs for permission checking
	chatIDs, err := rateLimitStore.GetAllGroups(ctx)
	if err != nil {
		logger.Warnw(
			"Failed to get groups for permission checking",
			"error", err,
		)
	}

	// Start periodic permission checker (FR-021: every 5 minutes)
	if len(chatIDs) > 0 {
		permChecker := telegram.NewPeriodicPermissionChecker(
			client,
			logger.SugaredLogger,
			5*time.Minute,
			onPermissionLost,
			onPermissionRestored,
		)
		go permChecker.Start(ctx, chatIDs, int64(botUserID))
		logger.Infow(
			"Periodic permission checker started",
			"chat_count", len(chatIDs),
			"check_interval", "5m",
		)
	}

	// Start health check HTTP server
	healthServer := startHealthCheckServer(cfg.Port, pgStore, logger)

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start processing updates
	updates := client.GetUpdatesChan()
	logger.Info("Bot is now running. Press CTRL-C to exit.")

	for {
		select {
		case update := <-updates:
			if err := handler.HandleUpdate(ctx, update); err != nil {
				logger.Errorw(
					"Error handling update",
					"error", err,
					"update_id", update.UpdateID,
				)
			}
		case <-sigChan:
			logger.Info("Shutdown signal received, exiting...")

			// Graceful shutdown
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := healthServer.Shutdown(shutdownCtx); err != nil {
				logger.Errorw(
					"Health check server shutdown error",
					"error", err,
				)
			}
			cancel()

			pgStore.Close()
			logger.Info("Bot shutdown complete")
			return
		}
	}
}

// runMigrations runs database migrations from embedded migration files
func runMigrations(databaseURL string, logger *logging.Logger) error {
	logger.Info("Running database migrations from embedded files...")

	// Open a connection for migrations
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database for migrations: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			logger.Errorw(
				"Error closing database connection",
				"error", closeErr,
			)
		}
	}()

	// Create postgres driver instance
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	// Create iofs source from embedded migrations
	sourceDriver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("failed to create iofs source driver: %w", err)
	}

	// Create migrate instance with embedded migrations
	m, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	// Run migrations
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	if errors.Is(err, migrate.ErrNoChange) {
		logger.Info("Database schema is up to date (embedded migrations)")
	} else {
		logger.Info("Database migrations completed successfully (from embedded files)")
	}

	return nil
}

// startHealthCheckServer starts an HTTP server for health checks
func startHealthCheckServer(port string, store *storage.PostgresStore, logger *logging.Logger) *http.Server {
	mux := http.NewServeMux()

	// Liveness probe - always returns 200 if the server is running
	mux.HandleFunc(
		"/health/live", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		},
	)

	// Readiness probe - checks database connectivity
	mux.HandleFunc(
		"/health/ready", func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()

			if err := store.Ping(ctx); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte("Database unavailable"))
				logger.Warnw(
					"Readiness check failed - database unavailable",
					"error", err,
				)
				return
			}

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Ready"))
		},
	)

	// Metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Infow(
			"Health check server started",
			"port", port,
			"endpoints", []string{"/health/live", "/health/ready", "/metrics"},
		)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Errorw(
				"Health check server error",
				"error", err,
			)
		}
	}()

	return server
}

// maskDatabaseURL masks sensitive parts of database URL for logging
func maskDatabaseURL(dbURL string) string {
	// Simple masking - in production, use more sophisticated approach
	if len(dbURL) > 20 {
		return dbURL[:10] + "***" + dbURL[len(dbURL)-5:]
	}
	return "***"
}
