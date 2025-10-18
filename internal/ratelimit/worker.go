// Package ratelimit provides rate limiting functionality for the Telegram bot.
// Clean implementation with ONLY cleanup functionality (no automatic unrestriction)
package ratelimit

import (
	"context"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Worker performs periodic cleanup tasks
// Note: Automatic unrestriction removed in simple messages design
type Worker struct {
	storage MultiWindowStorage
	bot     *tgbotapi.BotAPI
}

// NewWorker creates a new worker for periodic cleanup tasks
// Note: First parameter kept as interface{} for backward compatibility
func NewWorker(store interface{}, bot *tgbotapi.BotAPI) *Worker {
	// Try to cast to MultiWindowStorage
	multiStorage, ok := store.(MultiWindowStorage)
	if !ok {
		log.Printf("Warning: Worker requires MultiWindowStorage, got %T", store)
		return &Worker{bot: bot}
	}

	return &Worker{
		storage: multiStorage,
		bot:     bot,
	}
}

// Start begins the worker's periodic tasks
func (w *Worker) Start(ctx context.Context) {
	if w.storage == nil {
		log.Println("Worker: storage not available, worker disabled")
		return
	}

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	log.Println("Worker: started (cleanup only, no automatic unrestriction)")

	for {
		select {
		case <-ctx.Done():
			log.Println("Worker: stopped")
			return
		case <-ticker.C:
			if err := w.CleanupOldData(ctx); err != nil {
				log.Printf("Worker: cleanup failed: %v", err)
			}
		}
	}
}

// CleanupOldData removes old messages beyond maximum retention period
// and cleans up expired user overrides
// Keeps last 365 days of messages (longest possible window duration)
func (w *Worker) CleanupOldData(ctx context.Context) error {
	if w.storage == nil {
		return nil
	}

	// Clean up messages older than 365 days (max window duration)
	retentionSeconds := 365 * 24 * 3600
	if err := w.storage.CleanupOldMessages(ctx, retentionSeconds); err != nil {
		log.Printf("Worker: failed to cleanup old messages: %v", err)
	} else {
		log.Println("Worker: cleaned up old messages")
	}

	// Clean up expired overrides (where expires_at < NOW)
	if err := w.storage.CleanupExpiredOverrides(ctx); err != nil {
		log.Printf("Worker: failed to cleanup expired overrides: %v", err)
	} else {
		log.Println("Worker: cleaned up expired overrides")
	}

	return nil
}
