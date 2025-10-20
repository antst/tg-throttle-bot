// Package unit provides unit tests for mock storage functionality.
package unit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/antst/tg-throttle-bot/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMockStorage_UpsertGroupMembership tests the UpsertGroupMembership mock.
func TestMockStorage_UpsertGroupMembership(t *testing.T) {
	mockStore := NewMockStorage()
	ctx := context.Background()

	t.Run("successful upsert", func(t *testing.T) {
		mockStore.Reset()

		params := storage.UpsertGroupMembershipParams{
			ChatID:          -123456,
			UserID:          789012,
			Status:          "active",
			JoinedAt:        time.Now(),
			LeftAt:          nil,
			IsAdmin:         true,
			CanSendMessages: true,
		}

		membership, err := mockStore.UpsertGroupMembership(ctx, params)
		require.NoError(t, err)
		assert.Equal(t, params.ChatID, membership.ChatID)
		assert.Equal(t, params.UserID, membership.UserID)
		assert.Equal(t, params.Status, membership.Status)
		assert.True(t, membership.IsAdmin)

		// Verify call was tracked
		assert.Len(t, mockStore.UpsertGroupMembershipCalls, 1)
		assert.Equal(t, params.ChatID, mockStore.UpsertGroupMembershipCalls[0].ChatID)
	})

	t.Run("upsert with error", func(t *testing.T) {
		mockStore.Reset()

		// Configure mock to return error
		mockStore.UpsertGroupMembershipError = errors.New("database error")

		params := storage.UpsertGroupMembershipParams{
			ChatID:          -123456,
			UserID:          789012,
			Status:          "active",
			JoinedAt:        time.Now(),
			LeftAt:          nil,
			IsAdmin:         false,
			CanSendMessages: true,
		}

		_, err := mockStore.UpsertGroupMembership(ctx, params)
		assert.Error(t, err)
		assert.Equal(t, "database error", err.Error())

		// Verify call was still tracked
		assert.Len(t, mockStore.UpsertGroupMembershipCalls, 1)
	})

	t.Run("tracks multiple calls", func(t *testing.T) {
		mockStore.Reset()

		// Insert multiple memberships
		for i := int64(1); i <= 3; i++ {
			params := storage.UpsertGroupMembershipParams{
				ChatID:          -123456,
				UserID:          i,
				Status:          "active",
				JoinedAt:        time.Now(),
				LeftAt:          nil,
				IsAdmin:         false,
				CanSendMessages: true,
			}

			_, err := mockStore.UpsertGroupMembership(ctx, params)
			require.NoError(t, err)
		}

		assert.Len(t, mockStore.UpsertGroupMembershipCalls, 3)
		assert.Equal(t, int64(1), mockStore.UpsertGroupMembershipCalls[0].UserID)
		assert.Equal(t, int64(3), mockStore.UpsertGroupMembershipCalls[2].UserID)
	})
}

// TestMockStorage_CreateSyncMetadata tests the CreateSyncMetadata mock.
func TestMockStorage_CreateSyncMetadata(t *testing.T) {
	mockStore := NewMockStorage()
	ctx := context.Background()

	t.Run("successful creation", func(t *testing.T) {
		mockStore.Reset()

		chatID := int64(-123456)
		metadata, err := mockStore.CreateSyncMetadata(ctx, chatID)
		require.NoError(t, err)
		assert.Equal(t, chatID, metadata.ChatID)
		assert.Equal(t, "pending", metadata.SyncStatus)
		assert.Nil(t, metadata.LastSyncAt)

		// Verify call was tracked
		assert.Len(t, mockStore.CreateSyncMetadataCalls, 1)
		assert.Equal(t, chatID, mockStore.CreateSyncMetadataCalls[0])
	})

	t.Run("creation with error", func(t *testing.T) {
		mockStore.Reset()

		// Configure mock to return error
		mockStore.CreateSyncMetadataError = errors.New("database error")

		chatID := int64(-123456)
		_, err := mockStore.CreateSyncMetadata(ctx, chatID)
		assert.Error(t, err)
		assert.Equal(t, "database error", err.Error())

		// Verify call was still tracked
		assert.Len(t, mockStore.CreateSyncMetadataCalls, 1)
	})
}

// TestMockStorage_UpdateSyncMetadata tests the UpdateSyncMetadata mock.
func TestMockStorage_UpdateSyncMetadata(t *testing.T) {
	mockStore := NewMockStorage()
	ctx := context.Background()

	t.Run("successful update", func(t *testing.T) {
		mockStore.Reset()

		// First create metadata
		chatID := int64(-123456)
		_, err := mockStore.CreateSyncMetadata(ctx, chatID)
		require.NoError(t, err)

		// Then update it
		now := time.Now()
		nextSync := now.Add(24 * time.Hour)
		params := storage.UpdateSyncMetadataParams{
			ChatID:         chatID,
			SyncStatus:     "completed",
			LastSyncAt:     &now,
			NextSyncAt:     &nextSync,
			TotalMembers:   5,
			FailedAttempts: 0,
			LastError:      nil,
		}

		err = mockStore.UpdateSyncMetadata(ctx, params)
		require.NoError(t, err)

		// Verify call was tracked
		assert.Len(t, mockStore.UpdateSyncMetadataCalls, 1)
		assert.Equal(t, chatID, mockStore.UpdateSyncMetadataCalls[0].ChatID)
		assert.Equal(t, "completed", mockStore.UpdateSyncMetadataCalls[0].SyncStatus)
		assert.Equal(t, 5, mockStore.UpdateSyncMetadataCalls[0].TotalMembers)

		// Verify data was updated in mock storage
		updated := mockStore.SyncMetadata[chatID]
		assert.Equal(t, "completed", updated.SyncStatus)
		assert.Equal(t, 5, updated.TotalMembers)
	})

	t.Run("update with error", func(t *testing.T) {
		mockStore.Reset()

		// Configure mock to return error
		mockStore.UpdateSyncMetadataError = errors.New("update failed")

		params := storage.UpdateSyncMetadataParams{
			ChatID:     -123456,
			SyncStatus: "completed",
		}

		err := mockStore.UpdateSyncMetadata(ctx, params)
		assert.Error(t, err)
		assert.Equal(t, "update failed", err.Error())
	})
}

// TestMockStorage_RecordSyncEvent tests the RecordSyncEvent mock.
func TestMockStorage_RecordSyncEvent(t *testing.T) {
	mockStore := NewMockStorage()
	ctx := context.Background()

	t.Run("successful event recording", func(t *testing.T) {
		mockStore.Reset()

		startTime := time.Now()
		completedTime := startTime.Add(5 * time.Second)
		params := storage.RecordSyncEventParams{
			MetadataID:       1,
			EventType:        "initial_sync",
			StartedAt:        startTime,
			CompletedAt:      &completedTime,
			Status:           "success",
			ErrorMessage:     nil,
			MembersProcessed: 10,
			MembersAdded:     10,
			MembersUpdated:   0,
			MembersRemoved:   0,
		}

		event, err := mockStore.RecordSyncEvent(ctx, params)
		require.NoError(t, err)
		assert.Equal(t, params.MetadataID, event.MetadataID)
		assert.Equal(t, params.EventType, event.EventType)
		assert.Equal(t, params.Status, event.Status)
		assert.Equal(t, 10, event.MembersAdded)

		// Verify call was tracked
		assert.Len(t, mockStore.RecordSyncEventCalls, 1)
		assert.Equal(t, "initial_sync", mockStore.RecordSyncEventCalls[0].EventType)

		// Verify event was stored
		assert.Len(t, mockStore.SyncEvents, 1)
	})

	t.Run("record event with error", func(t *testing.T) {
		mockStore.Reset()

		// Configure mock to return error
		mockStore.RecordSyncEventError = errors.New("event recording failed")

		params := storage.RecordSyncEventParams{
			MetadataID: 1,
			EventType:  "periodic_sync",
			StartedAt:  time.Now(),
			Status:     "success",
		}

		_, err := mockStore.RecordSyncEvent(ctx, params)
		assert.Error(t, err)
		assert.Equal(t, "event recording failed", err.Error())
	})

	t.Run("tracks multiple events", func(t *testing.T) {
		mockStore.Reset()

		eventTypes := []string{"initial_sync", "periodic_sync", "notification_processed"}
		for _, eventType := range eventTypes {
			params := storage.RecordSyncEventParams{
				MetadataID: 1,
				EventType:  eventType,
				StartedAt:  time.Now(),
				Status:     "success",
			}

			_, err := mockStore.RecordSyncEvent(ctx, params)
			require.NoError(t, err)
		}

		assert.Len(t, mockStore.RecordSyncEventCalls, 3)
		assert.Len(t, mockStore.SyncEvents, 3)
		assert.Equal(t, "initial_sync", mockStore.SyncEvents[0].EventType)
		assert.Equal(t, "periodic_sync", mockStore.SyncEvents[1].EventType)
		assert.Equal(t, "notification_processed", mockStore.SyncEvents[2].EventType)
	})
}

// TestMockStorage_GetGroupsNeedingSync tests the GetGroupsNeedingSync mock.
func TestMockStorage_GetGroupsNeedingSync(t *testing.T) {
	mockStore := NewMockStorage()
	ctx := context.Background()

	t.Run("returns configured groups", func(t *testing.T) {
		mockStore.Reset()

		// Configure groups needing sync
		now := time.Now()
		mockStore.AddGroupNeedingSync(storage.SyncMetadata{
			ID:         1,
			ChatID:     -111,
			SyncStatus: "pending",
			NextSyncAt: &now,
		})
		mockStore.AddGroupNeedingSync(storage.SyncMetadata{
			ID:         2,
			ChatID:     -222,
			SyncStatus: "pending",
			NextSyncAt: &now,
		})

		groups, err := mockStore.GetGroupsNeedingSync(ctx)
		require.NoError(t, err)
		assert.Len(t, groups, 2)
		assert.Equal(t, int64(-111), groups[0].ChatID)
		assert.Equal(t, int64(-222), groups[1].ChatID)

		// Verify call was tracked
		assert.Equal(t, 1, mockStore.GetGroupsNeedingSyncCalls)
	})

	t.Run("returns empty list when no groups need sync", func(t *testing.T) {
		mockStore.Reset()

		groups, err := mockStore.GetGroupsNeedingSync(ctx)
		require.NoError(t, err)
		assert.Len(t, groups, 0)
	})

	t.Run("returns error when configured", func(t *testing.T) {
		mockStore.Reset()

		// Configure mock to return error
		mockStore.GetGroupsNeedingSyncError = errors.New("database error")

		_, err := mockStore.GetGroupsNeedingSync(ctx)
		assert.Error(t, err)
		assert.Equal(t, "database error", err.Error())
	})

	t.Run("tracks multiple calls", func(t *testing.T) {
		mockStore.Reset()

		// Call multiple times
		for i := 0; i < 3; i++ {
			_, _ = mockStore.GetGroupsNeedingSync(ctx)
		}

		assert.Equal(t, 3, mockStore.GetGroupsNeedingSyncCalls)
	})
}

// TestMockStorage_GetStaleGroupMemberships tests the GetStaleGroupMemberships mock.
func TestMockStorage_GetStaleGroupMemberships(t *testing.T) {
	mockStore := NewMockStorage()
	ctx := context.Background()

	t.Run("returns stale memberships for specific group", func(t *testing.T) {
		mockStore.Reset()

		chatID := int64(-123456)
		otherChatID := int64(-999999)

		// Add stale memberships for target group
		mockStore.AddStaleMembership(storage.GroupMembership{
			ChatID:    chatID,
			UserID:    1,
			Status:    "active",
			JoinedAt:  time.Now().Add(-72 * time.Hour),
			UpdatedAt: time.Now().Add(-72 * time.Hour),
		})
		mockStore.AddStaleMembership(storage.GroupMembership{
			ChatID:    chatID,
			UserID:    2,
			Status:    "active",
			JoinedAt:  time.Now().Add(-72 * time.Hour),
			UpdatedAt: time.Now().Add(-72 * time.Hour),
		})

		// Add stale membership for different group (should be filtered out)
		mockStore.AddStaleMembership(storage.GroupMembership{
			ChatID:    otherChatID,
			UserID:    999,
			Status:    "active",
			JoinedAt:  time.Now().Add(-72 * time.Hour),
			UpdatedAt: time.Now().Add(-72 * time.Hour),
		})

		staleMembers, err := mockStore.GetStaleGroupMemberships(ctx, chatID, 48*3600)
		require.NoError(t, err)
		assert.Len(t, staleMembers, 2)
		assert.Equal(t, chatID, staleMembers[0].ChatID)
		assert.Equal(t, chatID, staleMembers[1].ChatID)

		// Verify call was tracked
		assert.Len(t, mockStore.GetStaleMembershipsCalls, 1)
		assert.Equal(t, chatID, mockStore.GetStaleMembershipsCalls[0].ChatID)
		assert.Equal(t, int32(48*3600), mockStore.GetStaleMembershipsCalls[0].StaleDuration)
	})

	t.Run("returns empty list when no stale memberships", func(t *testing.T) {
		mockStore.Reset()

		chatID := int64(-123456)
		staleMembers, err := mockStore.GetStaleGroupMemberships(ctx, chatID, 48*3600)
		require.NoError(t, err)
		assert.Len(t, staleMembers, 0)
	})

	t.Run("returns error when configured", func(t *testing.T) {
		mockStore.Reset()

		// Configure mock to return error
		mockStore.GetStaleGroupMembershipsErr = errors.New("database error")

		_, err := mockStore.GetStaleGroupMemberships(ctx, -123456, 48*3600)
		assert.Error(t, err)
		assert.Equal(t, "database error", err.Error())
	})
}

// TestMockStorage_Reset tests the Reset functionality.
func TestMockStorage_Reset(t *testing.T) {
	mockStore := NewMockStorage()
	ctx := context.Background()

	// Add some data and calls
	_, _ = mockStore.CreateSyncMetadata(ctx, -123456)
	_, _ = mockStore.UpsertGroupMembership(ctx, storage.UpsertGroupMembershipParams{
		ChatID:   -123456,
		UserID:   1,
		Status:   "active",
		JoinedAt: time.Now(),
	})
	_, _ = mockStore.GetGroupsNeedingSync(ctx)

	assert.Len(t, mockStore.CreateSyncMetadataCalls, 1)
	assert.Len(t, mockStore.UpsertGroupMembershipCalls, 1)
	assert.Equal(t, 1, mockStore.GetGroupsNeedingSyncCalls)
	assert.Len(t, mockStore.SyncMetadata, 1)

	// Reset should clear everything
	mockStore.Reset()

	assert.Len(t, mockStore.CreateSyncMetadataCalls, 0)
	assert.Len(t, mockStore.UpsertGroupMembershipCalls, 0)
	assert.Equal(t, 0, mockStore.GetGroupsNeedingSyncCalls)
	assert.Len(t, mockStore.SyncMetadata, 0)
	assert.Len(t, mockStore.GroupMemberships, 0)
	assert.Len(t, mockStore.SyncEvents, 0)
}

// TestMockStorage_Concurrency tests thread-safety.
func TestMockStorage_Concurrency(t *testing.T) {
	mockStore := NewMockStorage()
	ctx := context.Background()

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			chatID := int64(-1000 - id)
			_, _ = mockStore.CreateSyncMetadata(ctx, chatID)
			_, _ = mockStore.UpsertGroupMembership(ctx, storage.UpsertGroupMembershipParams{
				ChatID:   chatID,
				UserID:   int64(id),
				Status:   "active",
				JoinedAt: time.Now(),
			})
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all calls were tracked (no race conditions)
	assert.Equal(t, 10, len(mockStore.CreateSyncMetadataCalls))
	assert.Equal(t, 10, len(mockStore.UpsertGroupMembershipCalls))
}
