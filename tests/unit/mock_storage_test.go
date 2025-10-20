// Package unit provides mock implementations for testing.
package unit

import (
	"context"
	"sync"
	"time"

	"github.com/antst/tg-throttle-bot/internal/ratelimit"
	"github.com/antst/tg-throttle-bot/internal/storage"
)

// MockStorage provides a mock implementation of storage.RateLimitStorage for testing.
// It tracks all method calls and allows configuring return values and errors.
type MockStorage struct {
	mu sync.RWMutex

	// Configuration for return values/errors
	UpsertGroupMembershipError  error
	CreateSyncMetadataError     error
	UpdateSyncMetadataError     error
	RecordSyncEventError        error
	GetGroupsNeedingSyncError   error
	GetStaleGroupMembershipsErr error

	// Mock data storage
	GroupMemberships  map[string]storage.GroupMembership // key: "chatID:userID"
	SyncMetadata      map[int64]storage.SyncMetadata     // key: chatID
	SyncEvents        []storage.SyncEvent
	GroupsNeedingSync []storage.SyncMetadata
	StaleMembers      []storage.GroupMembership

	// Call tracking
	UpsertGroupMembershipCalls []storage.UpsertGroupMembershipParams
	CreateSyncMetadataCalls    []int64
	UpdateSyncMetadataCalls    []storage.UpdateSyncMetadataParams
	RecordSyncEventCalls       []storage.RecordSyncEventParams
	GetGroupsNeedingSyncCalls  int
	GetStaleMembershipsCalls   []struct {
		ChatID        int64
		StaleDuration int32
	}
}

// NewMockStorage creates a new mock storage instance.
func NewMockStorage() *MockStorage {
	return &MockStorage{
		GroupMemberships:  make(map[string]storage.GroupMembership),
		SyncMetadata:      make(map[int64]storage.SyncMetadata),
		SyncEvents:        make([]storage.SyncEvent, 0),
		GroupsNeedingSync: make([]storage.SyncMetadata, 0),
		StaleMembers:      make([]storage.GroupMembership, 0),
	}
}

// Reset clears all tracked calls and mock data.
func (m *MockStorage) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GroupMemberships = make(map[string]storage.GroupMembership)
	m.SyncMetadata = make(map[int64]storage.SyncMetadata)
	m.SyncEvents = make([]storage.SyncEvent, 0)
	m.GroupsNeedingSync = make([]storage.SyncMetadata, 0)
	m.StaleMembers = make([]storage.GroupMembership, 0)

	m.UpsertGroupMembershipCalls = nil
	m.CreateSyncMetadataCalls = nil
	m.UpdateSyncMetadataCalls = nil
	m.RecordSyncEventCalls = nil
	m.GetGroupsNeedingSyncCalls = 0
	m.GetStaleMembershipsCalls = nil

	m.UpsertGroupMembershipError = nil
	m.CreateSyncMetadataError = nil
	m.UpdateSyncMetadataError = nil
	m.RecordSyncEventError = nil
	m.GetGroupsNeedingSyncError = nil
	m.GetStaleGroupMembershipsErr = nil
}

// UpsertGroupMembership mocks creating/updating a group membership.
func (m *MockStorage) UpsertGroupMembership(ctx context.Context, params storage.UpsertGroupMembershipParams) (storage.GroupMembership, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.UpsertGroupMembershipCalls = append(m.UpsertGroupMembershipCalls, params)

	if m.UpsertGroupMembershipError != nil {
		return storage.GroupMembership{}, m.UpsertGroupMembershipError
	}

	key := makeKey(params.ChatID, params.UserID)
	membership := storage.GroupMembership{
		ID:              int64(len(m.GroupMemberships) + 1),
		ChatID:          params.ChatID,
		UserID:          params.UserID,
		Status:          params.Status,
		JoinedAt:        params.JoinedAt,
		LeftAt:          params.LeftAt,
		UpdatedAt:       time.Now(),
		IsAdmin:         params.IsAdmin,
		CanSendMessages: params.CanSendMessages,
	}

	m.GroupMemberships[key] = membership
	return membership, nil
}

// CreateSyncMetadata mocks initializing sync metadata for a group.
func (m *MockStorage) CreateSyncMetadata(ctx context.Context, chatID int64) (storage.SyncMetadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CreateSyncMetadataCalls = append(m.CreateSyncMetadataCalls, chatID)

	if m.CreateSyncMetadataError != nil {
		return storage.SyncMetadata{}, m.CreateSyncMetadataError
	}

	metadata := storage.SyncMetadata{
		ID:             int64(len(m.SyncMetadata) + 1),
		ChatID:         chatID,
		SyncStatus:     "pending",
		LastSyncAt:     nil,
		NextSyncAt:     nil,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		TotalMembers:   0,
		FailedAttempts: 0,
		LastError:      nil,
	}

	m.SyncMetadata[chatID] = metadata
	return metadata, nil
}

// UpdateSyncMetadata mocks updating sync metadata.
func (m *MockStorage) UpdateSyncMetadata(ctx context.Context, params storage.UpdateSyncMetadataParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.UpdateSyncMetadataCalls = append(m.UpdateSyncMetadataCalls, params)

	if m.UpdateSyncMetadataError != nil {
		return m.UpdateSyncMetadataError
	}

	// Update existing metadata
	if existing, ok := m.SyncMetadata[params.ChatID]; ok {
		existing.SyncStatus = params.SyncStatus
		existing.LastSyncAt = params.LastSyncAt
		existing.NextSyncAt = params.NextSyncAt
		existing.TotalMembers = params.TotalMembers
		existing.FailedAttempts = params.FailedAttempts
		existing.LastError = params.LastError
		existing.UpdatedAt = time.Now()
		m.SyncMetadata[params.ChatID] = existing
	}

	return nil
}

// RecordSyncEvent mocks recording a sync event.
func (m *MockStorage) RecordSyncEvent(ctx context.Context, params storage.RecordSyncEventParams) (storage.SyncEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.RecordSyncEventCalls = append(m.RecordSyncEventCalls, params)

	if m.RecordSyncEventError != nil {
		return storage.SyncEvent{}, m.RecordSyncEventError
	}

	event := storage.SyncEvent{
		ID:               int64(len(m.SyncEvents) + 1),
		MetadataID:       params.MetadataID,
		EventType:        params.EventType,
		StartedAt:        params.StartedAt,
		CompletedAt:      params.CompletedAt,
		Status:           params.Status,
		ErrorMessage:     params.ErrorMessage,
		MembersProcessed: params.MembersProcessed,
		MembersAdded:     params.MembersAdded,
		MembersUpdated:   params.MembersUpdated,
		MembersRemoved:   params.MembersRemoved,
	}

	m.SyncEvents = append(m.SyncEvents, event)
	return event, nil
}

// GetGroupsNeedingSync mocks retrieving groups that need sync.
func (m *MockStorage) GetGroupsNeedingSync(ctx context.Context) ([]storage.SyncMetadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetGroupsNeedingSyncCalls++

	if m.GetGroupsNeedingSyncError != nil {
		return nil, m.GetGroupsNeedingSyncError
	}

	return m.GroupsNeedingSync, nil
}

// GetStaleGroupMemberships mocks retrieving stale memberships.
func (m *MockStorage) GetStaleGroupMemberships(ctx context.Context, chatID int64, staleDuration int32) ([]storage.GroupMembership, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetStaleMembershipsCalls = append(m.GetStaleMembershipsCalls, struct {
		ChatID        int64
		StaleDuration int32
	}{ChatID: chatID, StaleDuration: staleDuration})

	if m.GetStaleGroupMembershipsErr != nil {
		return nil, m.GetStaleGroupMembershipsErr
	}

	// Filter stale members by chatID
	result := make([]storage.GroupMembership, 0)
	for _, member := range m.StaleMembers {
		if member.ChatID == chatID {
			result = append(result, member)
		}
	}

	return result, nil
}

// SetGroupMembership adds a membership to the mock storage (for test setup).
func (m *MockStorage) SetGroupMembership(chatID, userID int64, membership storage.GroupMembership) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := makeKey(chatID, userID)
	m.GroupMemberships[key] = membership
}

// SetSyncMetadata adds sync metadata to the mock storage (for test setup).
func (m *MockStorage) SetSyncMetadata(chatID int64, metadata storage.SyncMetadata) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SyncMetadata[chatID] = metadata
}

// AddGroupNeedingSync adds a group to the list of groups needing sync (for test setup).
func (m *MockStorage) AddGroupNeedingSync(metadata storage.SyncMetadata) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GroupsNeedingSync = append(m.GroupsNeedingSync, metadata)
}

// AddStaleMembership adds a stale membership (for test setup).
func (m *MockStorage) AddStaleMembership(membership storage.GroupMembership) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.StaleMembers = append(m.StaleMembers, membership)
}

// makeKey creates a unique key for membership storage.
func makeKey(chatID, userID int64) string {
	return string(rune(chatID)) + ":" + string(rune(userID))
}

// Stub implementations for MultiWindowStorage interface methods
// (These are not used in sync coordinator tests but required for interface compliance)

func (m *MockStorage) EnsureUser(ctx context.Context, userID int64, username string) error {
	return nil
}

func (m *MockStorage) GetUserByUsername(ctx context.Context, username string) (int64, error) {
	return 0, nil
}

func (m *MockStorage) GetUsernameByUserID(ctx context.Context, userID int64) (string, error) {
	return "", nil
}

func (m *MockStorage) GetGroupByUsername(ctx context.Context, username string) (int64, error) {
	return 0, nil
}

func (m *MockStorage) EnsureGroupWithUsername(ctx context.Context, chatID int64, username string) error {
	return nil
}

func (m *MockStorage) CreateDefaultWindows(ctx context.Context, chatID int64) error {
	return nil
}

func (m *MockStorage) GetEnabledWindows(ctx context.Context, chatID int64) ([]*ratelimit.WindowSlot, error) {
	return nil, nil
}

func (m *MockStorage) GetAllWindows(ctx context.Context, chatID int64) ([]*ratelimit.WindowSlot, error) {
	return nil, nil
}

func (m *MockStorage) GetWindowSlot(ctx context.Context, chatID int64, slotID string) (*ratelimit.WindowSlot, error) {
	return nil, nil
}

func (m *MockStorage) UpdateWindowSlot(ctx context.Context, chatID int64, slotID string, charLimit int, durationValue int, durationUnit string, windowDuration int, enabled bool) error {
	return nil
}

func (m *MockStorage) SetWindowEnabled(ctx context.Context, chatID int64, slotID string, enabled bool) error {
	return nil
}

func (m *MockStorage) SetWindowSlot(ctx context.Context, chatID int64, slotID string, charLimit int, durationValue int, durationUnit string, windowDuration int, enabled bool) error {
	return nil
}

func (m *MockStorage) DisableWindow(ctx context.Context, chatID int64, slotID string) error {
	return nil
}

func (m *MockStorage) EnableWindow(ctx context.Context, chatID int64, slotID string) error {
	return nil
}

func (m *MockStorage) RecordMessage(ctx context.Context, userID, chatID int64, charCount int) error {
	return nil
}

func (m *MockStorage) GetWindowUsage(ctx context.Context, userID, chatID int64, windowSeconds int) (int, error) {
	return 0, nil
}

func (m *MockStorage) CleanupOldMessages(ctx context.Context, retentionSeconds int) error {
	return nil
}

func (m *MockStorage) ResetWindowMessages(ctx context.Context, userID, chatID int64) error {
	return nil
}

func (m *MockStorage) ResetWindowMessagesForAll(ctx context.Context, chatID int64) error {
	return nil
}

func (m *MockStorage) GetUserWindowStats(ctx context.Context, userID, chatID int64) ([]*ratelimit.UserWindowStat, error) {
	return nil, nil
}

func (m *MockStorage) IsGroupPaused(ctx context.Context, chatID int64) (bool, error) {
	return false, nil
}

func (m *MockStorage) GetGroupPausedUntil(ctx context.Context, chatID int64) (*time.Time, error) {
	return nil, nil
}

func (m *MockStorage) SetGroupPaused(ctx context.Context, chatID int64, paused bool, resumeAt *time.Time) error {
	return nil
}

func (m *MockStorage) GetUserOverride(ctx context.Context, chatID, userID int64) (*bool, error) {
	return nil, nil
}

func (m *MockStorage) SetUserOverride(ctx context.Context, chatID, userID int64, overrideState *bool, reason string, createdBy int64, expiresAt *time.Time) error {
	return nil
}

func (m *MockStorage) RemoveUserOverride(ctx context.Context, chatID, userID int64) error {
	return nil
}

func (m *MockStorage) GetAllOverrides(ctx context.Context, chatID int64) ([]*ratelimit.UserOverride, error) {
	return nil, nil
}

func (m *MockStorage) CleanupExpiredOverrides(ctx context.Context) error {
	return nil
}

func (m *MockStorage) GetGroupLanguage(ctx context.Context, chatID int64) (string, error) {
	return "en", nil
}

func (m *MockStorage) SetGroupLanguage(ctx context.Context, chatID int64, language string) error {
	return nil
}

func (m *MockStorage) ListAllGroupLanguages(ctx context.Context) (map[int64]string, error) {
	return nil, nil
}

func (m *MockStorage) EnsureGroup(ctx context.Context, chatID int64) error {
	return nil
}

func (m *MockStorage) GetUserLanguage(ctx context.Context, userID int64) (*string, error) {
	return nil, nil
}

func (m *MockStorage) SetUserLanguage(ctx context.Context, userID int64, language string) error {
	return nil
}

func (m *MockStorage) GetAllGroups(ctx context.Context) ([]int64, error) {
	return nil, nil
}
