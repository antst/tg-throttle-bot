// Package telegram provides Telegram Bot API client wrapper and helper functions.
package telegram

import (
	"context"
	"fmt"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// MockClient is a mock implementation of Client interface for testing.
type MockClient struct {
	mu sync.RWMutex

	// Configurable responses
	IsAdminResponse            map[int64]map[int64]bool // chatID -> userID -> isAdmin
	IsCreatorResponse          map[int64]map[int64]bool // chatID -> userID -> isCreator
	GetUserRolesResponse       map[int64]map[int64][]string
	GetChatMemberResponse      map[int64]map[int64]tgbotapi.ChatMember
	CheckBotPermissionsResp    map[int64]BotPermissions
	SendMessageError           error
	SendReplyError             error
	DeleteMessageError         error
	RestrictUserError          error
	UnrestrictUserError        error
	SendAdminNotificationError error

	// Call tracking
	DeletedMessages       []DeletedMessage
	SentMessages          []SentMessage
	SentReplies           []SentReply
	SentEphemeralMessages []SentEphemeralMessage
	RestrictedUsers       []RestrictedUser
	UnrestrictedUsers     []UnrestrictedUser
	AdminNotifications    []AdminNotification
	IsAdminCalls          []AdminCheck
	IsCreatorCalls        []AdminCheck
	GetUserRolesCalls     []AdminCheck
	GetChatMemberCalls    []AdminCheck
	CheckPermissionsCalls []int64
}

// SentEphemeralMessage tracks an ephemeral message call
type SentEphemeralMessage struct {
	ChatID int64
	Text   string
	Config *EphemeralMessageConfig
}

// DeletedMessage tracks a deleted message call.
type DeletedMessage struct {
	ChatID    int64
	MessageID int
}

// SentMessage tracks a sent message call.
type SentMessage struct {
	ChatID int64
	Text   string
}

// SentReply tracks a sent reply call.
type SentReply struct {
	ChatID           int64
	ReplyToMessageID int
	Text             string
}

// RestrictedUser tracks a restrict user call.
type RestrictedUser struct {
	ChatID int64
	UserID int64
}

// UnrestrictedUser tracks an unrestrict user call.
type UnrestrictedUser struct {
	ChatID int64
	UserID int64
}

// AdminNotification tracks an admin notification call.
type AdminNotification struct {
	ChatID int64
	Text   string
}

// AdminCheck tracks an admin/creator/roles check call.
type AdminCheck struct {
	ChatID int64
	UserID int64
}

// BotPermissions holds bot permission check results.
type BotPermissions struct {
	CanRestrict bool
	CanDelete   bool
	Error       error
}

// NewMockClient creates a new mock Telegram client.
func NewMockClient() *MockClient {
	return &MockClient{
		IsAdminResponse:         make(map[int64]map[int64]bool),
		IsCreatorResponse:       make(map[int64]map[int64]bool),
		GetUserRolesResponse:    make(map[int64]map[int64][]string),
		GetChatMemberResponse:   make(map[int64]map[int64]tgbotapi.ChatMember),
		CheckBotPermissionsResp: make(map[int64]BotPermissions),
		DeletedMessages:         []DeletedMessage{},
		SentMessages:            []SentMessage{},
		SentReplies:             []SentReply{},
		RestrictedUsers:         []RestrictedUser{},
		UnrestrictedUsers:       []UnrestrictedUser{},
		AdminNotifications:      []AdminNotification{},
		IsAdminCalls:            []AdminCheck{},
		IsCreatorCalls:          []AdminCheck{},
		GetUserRolesCalls:       []AdminCheck{},
		GetChatMemberCalls:      []AdminCheck{},
		CheckPermissionsCalls:   []int64{},
	}
}

// SetAdminStatus configures the mock to return a specific admin status.
func (m *MockClient) SetAdminStatus(chatID, userID int64, isAdmin bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.IsAdminResponse[chatID] == nil {
		m.IsAdminResponse[chatID] = make(map[int64]bool)
	}
	m.IsAdminResponse[chatID][userID] = isAdmin
}

// SetCreatorStatus configures the mock to return a specific creator status.
func (m *MockClient) SetCreatorStatus(chatID, userID int64, isCreator bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.IsCreatorResponse[chatID] == nil {
		m.IsCreatorResponse[chatID] = make(map[int64]bool)
	}
	m.IsCreatorResponse[chatID][userID] = isCreator
}

// SetUserRoles configures the mock to return specific roles for a user.
func (m *MockClient) SetUserRoles(chatID, userID int64, roles []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.GetUserRolesResponse[chatID] == nil {
		m.GetUserRolesResponse[chatID] = make(map[int64][]string)
	}
	m.GetUserRolesResponse[chatID][userID] = roles
}

// SetChatMember configures the mock to return a specific chat member.
func (m *MockClient) SetChatMember(chatID, userID int64, member tgbotapi.ChatMember) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.GetChatMemberResponse[chatID] == nil {
		m.GetChatMemberResponse[chatID] = make(map[int64]tgbotapi.ChatMember)
	}
	m.GetChatMemberResponse[chatID][userID] = member
}

// SetBotPermissions configures the mock to return specific bot permissions.
func (m *MockClient) SetBotPermissions(chatID int64, canRestrict, canDelete bool, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CheckBotPermissionsResp[chatID] = BotPermissions{
		CanRestrict: canRestrict,
		CanDelete:   canDelete,
		Error:       err,
	}
}

// GetUpdatesChan returns an empty channel (not used in most tests).
func (m *MockClient) GetUpdatesChan() tgbotapi.UpdatesChannel {
	ch := make(chan tgbotapi.Update)
	return ch
}

// GetBotAPI returns nil (mock doesn't have a real bot).
func (m *MockClient) GetBotAPI() *tgbotapi.BotAPI {
	return nil
}

// GetBot returns nil (mock doesn't have a real bot).
func (m *MockClient) GetBot() *tgbotapi.BotAPI {
	return nil
}

// DeleteMessage tracks the delete message call.
func (m *MockClient) DeleteMessage(chatID int64, messageID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.DeleteMessageError != nil {
		return m.DeleteMessageError
	}

	m.DeletedMessages = append(
		m.DeletedMessages, DeletedMessage{
			ChatID:    chatID,
			MessageID: messageID,
		},
	)
	return nil
}

// SendMessage tracks the send message call.
func (m *MockClient) SendMessage(chatID int64, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.SendMessageError != nil {
		return m.SendMessageError
	}

	m.SentMessages = append(
		m.SentMessages, SentMessage{
			ChatID: chatID,
			Text:   text,
		},
	)
	return nil
}

// SendReply tracks the send reply call.
func (m *MockClient) SendReply(chatID int64, replyToMessageID int, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.SendReplyError != nil {
		return m.SendReplyError
	}

	m.SentReplies = append(
		m.SentReplies, SentReply{
			ChatID:           chatID,
			ReplyToMessageID: replyToMessageID,
			Text:             text,
		},
	)
	return nil
}

// IsAdmin returns the configured admin status.
func (m *MockClient) IsAdmin(_ context.Context, chatID int64, userID int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.IsAdminCalls = append(
		m.IsAdminCalls, AdminCheck{
			ChatID: chatID,
			UserID: userID,
		},
	)

	if chatUsers, ok := m.IsAdminResponse[chatID]; ok {
		if isAdmin, ok := chatUsers[userID]; ok {
			return isAdmin, nil
		}
	}

	// Default: return false for unknown users
	return false, nil
}

// IsCreator returns the configured creator status.
func (m *MockClient) IsCreator(_ context.Context, chatID int64, userID int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.IsCreatorCalls = append(
		m.IsCreatorCalls, AdminCheck{
			ChatID: chatID,
			UserID: userID,
		},
	)

	if chatUsers, ok := m.IsCreatorResponse[chatID]; ok {
		if isCreator, ok := chatUsers[userID]; ok {
			return isCreator, nil
		}
	}

	// Default: return false for unknown users
	return false, nil
}

// GetUserRoles returns the configured roles.
func (m *MockClient) GetUserRoles(_ context.Context, chatID int64, userID int64) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetUserRolesCalls = append(
		m.GetUserRolesCalls, AdminCheck{
			ChatID: chatID,
			UserID: userID,
		},
	)

	if chatUsers, ok := m.GetUserRolesResponse[chatID]; ok {
		if roles, ok := chatUsers[userID]; ok {
			return roles, nil
		}
	}

	// Default: return empty roles
	return []string{}, nil
}

// GetChatMember returns the configured chat member.
func (m *MockClient) GetChatMember(_ context.Context, chatID int64, userID int64) (tgbotapi.ChatMember, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetChatMemberCalls = append(
		m.GetChatMemberCalls, AdminCheck{
			ChatID: chatID,
			UserID: userID,
		},
	)

	if chatUsers, ok := m.GetChatMemberResponse[chatID]; ok {
		if member, ok := chatUsers[userID]; ok {
			return member, nil
		}
	}

	// Default: return error for unknown users
	return tgbotapi.ChatMember{}, fmt.Errorf("chat member not found")
}

// SendAdminNotification tracks the admin notification call.
func (m *MockClient) SendAdminNotification(_ context.Context, chatID int64, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.SendAdminNotificationError != nil {
		return m.SendAdminNotificationError
	}

	m.AdminNotifications = append(
		m.AdminNotifications, AdminNotification{
			ChatID: chatID,
			Text:   text,
		},
	)
	return nil
}

// RestrictUser tracks the restrict user call.
func (m *MockClient) RestrictUser(_ context.Context, chatID int64, userID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.RestrictUserError != nil {
		return m.RestrictUserError
	}

	m.RestrictedUsers = append(
		m.RestrictedUsers, RestrictedUser{
			ChatID: chatID,
			UserID: userID,
		},
	)
	return nil
}

// UnrestrictUser tracks the unrestrict user call.
func (m *MockClient) UnrestrictUser(_ context.Context, chatID int64, userID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.UnrestrictUserError != nil {
		return m.UnrestrictUserError
	}

	m.UnrestrictedUsers = append(
		m.UnrestrictedUsers, UnrestrictedUser{
			ChatID: chatID,
			UserID: userID,
		},
	)
	return nil
}

// CheckBotPermissions returns the configured permissions.
func (m *MockClient) CheckBotPermissions(_ context.Context, chatID int64) (bool, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CheckPermissionsCalls = append(m.CheckPermissionsCalls, chatID)

	if perms, ok := m.CheckBotPermissionsResp[chatID]; ok {
		return perms.CanRestrict, perms.CanDelete, perms.Error
	}

	// Default: bot has all permissions
	return true, true, nil
}

// SendEphemeralMessage tracks the ephemeral message call and returns a mock message
func (m *MockClient) SendEphemeralMessage(chatID int64, text string, config *EphemeralMessageConfig) (tgbotapi.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SentEphemeralMessages = append(m.SentEphemeralMessages, SentEphemeralMessage{
		ChatID: chatID,
		Text:   text,
		Config: config,
	})

	// Return a mock message
	return tgbotapi.Message{
		MessageID: 12345,
		Chat: &tgbotapi.Chat{
			ID: chatID,
		},
		Text: text,
	}, nil
}

// Reset clears all tracked calls and responses.
func (m *MockClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.DeletedMessages = []DeletedMessage{}
	m.SentMessages = []SentMessage{}
	m.SentReplies = []SentReply{}
	m.SentEphemeralMessages = []SentEphemeralMessage{}
	m.RestrictedUsers = []RestrictedUser{}
	m.UnrestrictedUsers = []UnrestrictedUser{}
	m.AdminNotifications = []AdminNotification{}
	m.IsAdminCalls = []AdminCheck{}
	m.IsCreatorCalls = []AdminCheck{}
	m.GetUserRolesCalls = []AdminCheck{}
	m.GetChatMemberCalls = []AdminCheck{}
	m.CheckPermissionsCalls = []int64{}
}
