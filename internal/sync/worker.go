// Package sync provides proactive member synchronization for Telegram groups.
package sync

import (
	"context"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/antst/tg-throttle-bot/internal/storage"
)

// PeriodicSyncWorker runs background sync operations on a fixed schedule.
type PeriodicSyncWorker struct {
	coordinator *SyncCoordinator
	store       PeriodicSyncStorage
	logger      *zap.Logger
	interval    time.Duration
	stopChan    chan struct{}
}

// PeriodicSyncStorage defines storage operations needed for periodic sync.
type PeriodicSyncStorage interface {
	GetGroupsNeedingSync(ctx context.Context) ([]storage.SyncMetadata, error)
	GetStaleGroupMemberships(ctx context.Context, chatID int64, staleDuration int32) ([]storage.GroupMembership, error)
}

// NewPeriodicSyncWorker creates a new periodic sync worker.
func NewPeriodicSyncWorker(coordinator *SyncCoordinator, store PeriodicSyncStorage, logger *zap.Logger, interval time.Duration) *PeriodicSyncWorker {
	return &PeriodicSyncWorker{
		coordinator: coordinator,
		store:       store,
		logger:      logger,
		interval:    interval,
		stopChan:    make(chan struct{}),
	}
}

// Start begins the periodic sync worker.
func (w *PeriodicSyncWorker) Start(ctx context.Context) {
	w.logger.Info("Periodic sync worker started", zap.Duration("interval", w.interval))

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Run initial sync immediately
	w.runSync(ctx)

	for {
		select {
		case <-ticker.C:
			w.runSync(ctx)
		case <-w.stopChan:
			w.logger.Info("Periodic sync worker stopped")
			return
		case <-ctx.Done():
			w.logger.Info("Periodic sync worker stopped (context cancelled)")
			return
		}
	}
}

// Stop halts the periodic sync worker.
func (w *PeriodicSyncWorker) Stop() {
	close(w.stopChan)
}

// runSync performs a single sync iteration for all groups needing sync.
func (w *PeriodicSyncWorker) runSync(ctx context.Context) {
	w.logger.Info("Starting periodic sync iteration")
	startTime := time.Now()

	// Get groups that need sync (next_sync_at <= NOW)
	groups, err := w.store.GetGroupsNeedingSync(ctx)
	if err != nil {
		w.logger.Error("Failed to get groups needing sync", zap.Error(err))
		return
	}

	if len(groups) == 0 {
		w.logger.Debug("No groups need sync at this time")
		return
	}

	w.logger.Info("Found groups needing sync", zap.Int("count", len(groups)))

	syncedCount := 0
	failedCount := 0

	for _, metadata := range groups {
		// Use coordinator's PeriodicGroupSync method
		if err := w.coordinator.PeriodicGroupSync(ctx, metadata.ChatID); err != nil {
			w.logger.Warn("Failed to sync group",
				zap.Int64("chat_id", metadata.ChatID),
				zap.Error(err),
			)
			failedCount++
		} else {
			syncedCount++
		}

		// Update stale record count metric (48 hours = 172800 seconds)
		staleRecords, err := w.store.GetStaleGroupMemberships(ctx, metadata.ChatID, 172800)
		if err != nil {
			w.logger.Warn("Failed to get stale group memberships",
				zap.Int64("chat_id", metadata.ChatID),
				zap.Error(err),
			)
		} else {
			// Update gauge with stale record count for this group
			MemberSyncStaleRecords.WithLabelValues(
				strconv.FormatInt(metadata.ChatID, 10),
			).Set(float64(len(staleRecords)))

			if len(staleRecords) > 0 {
				w.logger.Debug("Found stale membership records",
					zap.Int64("chat_id", metadata.ChatID),
					zap.Int("stale_count", len(staleRecords)),
				)
			}
		}

		// Small delay between syncs to avoid rate limiting
		time.Sleep(100 * time.Millisecond)
	}

	duration := time.Since(startTime)
	w.logger.Info("Periodic sync iteration complete",
		zap.Int("synced", syncedCount),
		zap.Int("failed", failedCount),
		zap.Duration("duration", duration),
	)
}
