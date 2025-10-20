// Package unit provides unit tests for telegram client functionality.
package unit

import (
	"context"
	"testing"

	"github.com/antst/tg-throttle-bot/internal/telegram"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMockClient verifies the mock client implementation works correctly.
func TestMockClient(t *testing.T) {
	mockClient := telegram.NewMockClient()
	ctx := context.Background()

	t.Run(
		"tracks admin status correctly", func(t *testing.T) {
			chatID := int64(123)
			userID := int64(456)

			mockClient.SetAdminStatus(chatID, userID, true)

			isAdmin, err := mockClient.IsAdmin(ctx, chatID, userID)
			require.NoError(t, err)
			assert.True(t, isAdmin)

			// Verify call was tracked
			assert.Len(t, mockClient.IsAdminCalls, 1)
			assert.Equal(t, chatID, mockClient.IsAdminCalls[0].ChatID)
			assert.Equal(t, userID, mockClient.IsAdminCalls[0].UserID)
		},
	)

	t.Run(
		"tracks creator status correctly", func(t *testing.T) {
			mockClient.Reset()

			chatID := int64(789)
			userID := int64(101)

			mockClient.SetCreatorStatus(chatID, userID, true)

			isCreator, err := mockClient.IsCreator(ctx, chatID, userID)
			require.NoError(t, err)
			assert.True(t, isCreator)

			// Verify call was tracked
			assert.Len(t, mockClient.IsCreatorCalls, 1)
		},
	)

	t.Run(
		"tracks sent messages", func(t *testing.T) {
			mockClient.Reset()

			chatID := int64(111)
			text := "Test message"

			err := mockClient.SendMessage(chatID, text)
			require.NoError(t, err)

			assert.Len(t, mockClient.SentMessages, 1)
			assert.Equal(t, chatID, mockClient.SentMessages[0].ChatID)
			assert.Equal(t, text, mockClient.SentMessages[0].Text)
		},
	)

	t.Run(
		"tracks deleted messages", func(t *testing.T) {
			mockClient.Reset()

			chatID := int64(222)
			messageID := 333

			err := mockClient.DeleteMessage(chatID, messageID)
			require.NoError(t, err)

			assert.Len(t, mockClient.DeletedMessages, 1)
			assert.Equal(t, chatID, mockClient.DeletedMessages[0].ChatID)
			assert.Equal(t, messageID, mockClient.DeletedMessages[0].MessageID)
		},
	)

	t.Run(
		"tracks restricted users", func(t *testing.T) {
			mockClient.Reset()

			chatID := int64(444)
			userID := int64(555)

			err := mockClient.RestrictUser(ctx, chatID, userID)
			require.NoError(t, err)

			assert.Len(t, mockClient.RestrictedUsers, 1)
			assert.Equal(t, chatID, mockClient.RestrictedUsers[0].ChatID)
			assert.Equal(t, userID, mockClient.RestrictedUsers[0].UserID)
		},
	)

	t.Run(
		"tracks unrestricted users", func(t *testing.T) {
			mockClient.Reset()

			chatID := int64(666)
			userID := int64(777)

			err := mockClient.UnrestrictUser(ctx, chatID, userID)
			require.NoError(t, err)

			assert.Len(t, mockClient.UnrestrictedUsers, 1)
			assert.Equal(t, chatID, mockClient.UnrestrictedUsers[0].ChatID)
			assert.Equal(t, userID, mockClient.UnrestrictedUsers[0].UserID)
		},
	)

	t.Run(
		"returns configured bot permissions", func(t *testing.T) {
			mockClient.Reset()

			chatID := int64(888)
			mockClient.SetBotPermissions(chatID, true, true, nil)

			canRestrict, canDelete, err := mockClient.CheckBotPermissions(ctx, chatID)
			require.NoError(t, err)
			assert.True(t, canRestrict)
			assert.True(t, canDelete)

			assert.Len(t, mockClient.CheckPermissionsCalls, 1)
			assert.Equal(t, chatID, mockClient.CheckPermissionsCalls[0])
		},
	)

	t.Run(
		"returns false for unknown users by default", func(t *testing.T) {
			mockClient.Reset()

			chatID := int64(999)
			unknownUserID := int64(1000)

			isAdmin, err := mockClient.IsAdmin(ctx, chatID, unknownUserID)
			require.NoError(t, err)
			assert.False(t, isAdmin, "unknown users should not be admin by default")
		},
	)

	t.Run(
		"reset clears all tracked calls", func(t *testing.T) {
			mockClient.Reset()

			// Add some data
			mockClient.SetAdminStatus(123, 456, true)
			_, _ = mockClient.IsAdmin(ctx, 123, 456)
			_ = mockClient.SendMessage(123, "test")

			assert.Len(t, mockClient.IsAdminCalls, 1)
			assert.Len(t, mockClient.SentMessages, 1)

			// Reset should clear everything
			mockClient.Reset()

			assert.Len(t, mockClient.IsAdminCalls, 0)
			assert.Len(t, mockClient.SentMessages, 0)
			assert.Len(t, mockClient.DeletedMessages, 0)
			assert.Len(t, mockClient.RestrictedUsers, 0)
		},
	)

	t.Run(
		"can simulate errors", func(t *testing.T) {
			mockClient.Reset()

			// Configure mock to return error
			mockClient.SendMessageError = assert.AnError

			err := mockClient.SendMessage(123, "test")
			assert.Error(t, err)
			assert.Len(t, mockClient.SentMessages, 0, "failed calls should not be tracked")
		},
	)

	t.Run(
		"tracks user roles", func(t *testing.T) {
			mockClient.Reset()

			chatID := int64(111)
			userID := int64(222)
			expectedRoles := []string{"admin", "creator"}

			mockClient.SetUserRoles(chatID, userID, expectedRoles)

			roles, err := mockClient.GetUserRoles(ctx, chatID, userID)
			require.NoError(t, err)
			assert.Equal(t, expectedRoles, roles)

			assert.Len(t, mockClient.GetUserRolesCalls, 1)
		},
	)

	t.Run(
		"tracks replies", func(t *testing.T) {
			mockClient.Reset()

			chatID := int64(333)
			replyToID := 444
			text := "Reply text"

			err := mockClient.SendReply(chatID, replyToID, text)
			require.NoError(t, err)

			assert.Len(t, mockClient.SentReplies, 1)
			assert.Equal(t, chatID, mockClient.SentReplies[0].ChatID)
			assert.Equal(t, replyToID, mockClient.SentReplies[0].ReplyToMessageID)
			assert.Equal(t, text, mockClient.SentReplies[0].Text)
		},
	)
}

// TestMockClientConcurrency verifies mock client is thread-safe.
func TestMockClientConcurrency(t *testing.T) {
	mockClient := telegram.NewMockClient()
	ctx := context.Background()

	// Run multiple goroutines concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			chatID := int64(id)
			userID := int64(id + 100)

			mockClient.SetAdminStatus(chatID, userID, true)
			_, _ = mockClient.IsAdmin(ctx, chatID, userID)
			_ = mockClient.SendMessage(chatID, "test")

			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all calls were tracked (no race conditions)
	assert.Equal(t, 10, len(mockClient.IsAdminCalls))
	assert.Equal(t, 10, len(mockClient.SentMessages))
}
