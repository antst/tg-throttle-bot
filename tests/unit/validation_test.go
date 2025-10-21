package unit

import (
	"fmt"
	"testing"

	"github.com/antst/tg-throttle-bot/internal/bot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockGroupResolver implements bot.GroupResolver for testing
type mockGroupResolver struct {
	resolveUsernameFunc func(username string) (int64, error)
	resolveTitleFunc    func(title string) ([]int64, error)
	getGroupInfoFunc    func(chatID int64) (username, title *string, err error)
}

func (m *mockGroupResolver) ResolveUsername(username string) (int64, error) {
	if m.resolveUsernameFunc != nil {
		return m.resolveUsernameFunc(username)
	}
	return 0, fmt.Errorf("ResolveUsername not implemented")
}

func (m *mockGroupResolver) ResolveTitle(title string) ([]int64, error) {
	if m.resolveTitleFunc != nil {
		return m.resolveTitleFunc(title)
	}
	return nil, fmt.Errorf("ResolveTitle not implemented")
}

func (m *mockGroupResolver) GetGroupInfo(chatID int64) (username, title *string, err error) {
	if m.getGroupInfoFunc != nil {
		return m.getGroupInfoFunc(chatID)
	}
	return nil, nil, fmt.Errorf("GetGroupInfo not implemented")
}

// TestParseGroupIdentifier_Username tests @username parsing (T033)
func TestParseGroupIdentifier_Username(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expectError bool
		expected    *bot.GroupIdentifier
	}{
		{
			name:        "valid username with @",
			input:       "@home",
			expectError: false,
			expected: &bot.GroupIdentifier{
				Username: "home",
				IsName:   true,
			},
		},
		{
			name:        "valid username with underscores",
			input:       "@work_group",
			expectError: false,
			expected: &bot.GroupIdentifier{
				Username: "work_group",
				IsName:   true,
			},
		},
		{
			name:        "valid username with numbers",
			input:       "@team2024",
			expectError: false,
			expected: &bot.GroupIdentifier{
				Username: "team2024",
				IsName:   true,
			},
		},
		{
			name:        "empty username after @",
			input:       "@",
			expectError: true,
		},
		{
			name:        "username too short",
			input:       "@ab",
			expectError: true,
		},
		{
			name:        "username too long",
			input:       "@" + string(make([]byte, 33)),
			expectError: true,
		},
		{
			name:        "username with invalid chars",
			input:       "@home-group",
			expectError: true,
		},
		{
			name:        "username with spaces",
			input:       "@home group",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := bot.ParseGroupIdentifier(tc.input)

			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tc.expected.Username, result.Username)
				assert.Equal(t, tc.expected.IsName, result.IsName)
				assert.True(t, result.IsName, "Should be marked as username")
			}
		})
	}
}

// TestParseGroupIdentifier_NumericID tests numeric Group ID parsing (T034)
func TestParseGroupIdentifier_NumericID(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expectError bool
		expected    *bot.GroupIdentifier
	}{
		{
			name:        "valid negative group ID",
			input:       "-4211780967",
			expectError: false,
			expected: &bot.GroupIdentifier{
				GroupID: -4211780967,
				IsName:  false,
			},
		},
		{
			name:        "valid supergroup ID",
			input:       "-1001234567890",
			expectError: false,
			expected: &bot.GroupIdentifier{
				GroupID: -1001234567890,
				IsName:  false,
			},
		},
		{
			name:        "positive number invalid",
			input:       "123456",
			expectError: true,
		},
		{
			name:        "zero invalid",
			input:       "0",
			expectError: true,
		},
		{
			name:  "non-numeric string treated as title",
			input: "not-a-number",
			expected: &bot.GroupIdentifier{
				Title:   "not-a-number",
				IsTitle: true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := bot.ParseGroupIdentifier(tc.input)

			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				if tc.expected.GroupID != 0 {
					assert.Equal(t, tc.expected.GroupID, result.GroupID)
					assert.False(t, result.IsName, "Should be marked as numeric ID")
				} else if tc.expected.Username != "" {
					assert.Equal(t, tc.expected.Username, result.Username)
					assert.True(t, result.IsName, "Should be marked as username")
				} else if tc.expected.Title != "" {
					assert.Equal(t, tc.expected.Title, result.Title)
					assert.True(t, result.IsTitle, "Should be marked as title")
				}
			}
		})
	}
}

// TestParseGroupIdentifier_Validation tests edge cases and validation (T035)
func TestParseGroupIdentifier_Validation(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expectError bool
		expected    *bot.GroupIdentifier
		errorMsg    string
	}{
		{
			name:        "empty string",
			input:       "",
			expectError: true,
			errorMsg:    "group identifier cannot be empty",
		},
		{
			name:        "whitespace only",
			input:       "   ",
			expectError: true,
			errorMsg:    "group identifier cannot be empty",
		},
		{
			name:        "@ with whitespace",
			input:       "@  ",
			expectError: true,
			errorMsg:    "username cannot be empty after @",
		},
		{
			name:  "text without @ treated as title",
			input: "home",
			expected: &bot.GroupIdentifier{
				Title:   "home",
				IsTitle: true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := bot.ParseGroupIdentifier(tc.input)

			if tc.expectError {
				require.Error(t, err)
				assert.Nil(t, result)
				if tc.errorMsg != "" {
					assert.Contains(t, err.Error(), tc.errorMsg)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tc.expected.Title, result.Title)
				assert.True(t, result.IsTitle)
			}
		})
	}
}

// TestParseGroupIdentifier_RealWorldExamples tests with actual Telegram group identifiers
func TestParseGroupIdentifier_RealWorldExamples(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected bot.GroupIdentifier
	}{
		{
			name:  "home group from live testing",
			input: "-4211780967",
			expected: bot.GroupIdentifier{
				GroupID: -4211780967,
				IsName:  false,
			},
		},
		{
			name:  "@home username",
			input: "@home",
			expected: bot.GroupIdentifier{
				Username: "home",
				IsName:   true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := bot.ParseGroupIdentifier(tc.input)

			require.NoError(t, err)
			require.NotNil(t, result)

			if tc.expected.IsName {
				assert.Equal(t, tc.expected.Username, result.Username)
				assert.True(t, result.IsName)
			} else {
				assert.Equal(t, tc.expected.GroupID, result.GroupID)
				assert.False(t, result.IsName)
			}
		})
	}
}

// TestResolveTargetGroup tests group parameter validation and context resolution (T036)
func TestResolveTargetGroup(t *testing.T) {
	t.Run("private chat requires group parameter", func(t *testing.T) {
		privateChatID := int64(290924591) // Positive = private chat
		args := []string{}                // No arguments

		result, remainingArgs, err := bot.ResolveTargetGroup(privateChatID, args)

		// Should fail because no group identifier provided
		require.Error(t, err)
		assert.Contains(t, err.Error(), "when using commands from private chat")
		assert.Nil(t, result)
		assert.Nil(t, remainingArgs)
	})

	t.Run("private chat with single-word title identifier", func(t *testing.T) {
		privateChatID := int64(290924591)
		args := []string{"home", "a", "1000", "30m"}

		result, remainingArgs, err := bot.ResolveTargetGroup(privateChatID, args)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "home", result.Title)
		assert.True(t, result.IsTitle)
		assert.Equal(t, []string{"a", "1000", "30m"}, remainingArgs)
	})

	t.Run("private chat with @username parameter", func(t *testing.T) {
		privateChatID := int64(290924591)
		args := []string{"@home", "a", "1000", "30m"}

		result, remainingArgs, err := bot.ResolveTargetGroup(privateChatID, args)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.IsName)
		assert.Equal(t, "home", result.Username)
		assert.Equal(t, []string{"a", "1000", "30m"}, remainingArgs)
	})

	t.Run("private chat with numeric ID parameter", func(t *testing.T) {
		privateChatID := int64(290924591)
		args := []string{"-4211780967", "a", "1000", "30m"}

		result, remainingArgs, err := bot.ResolveTargetGroup(privateChatID, args)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.IsName)
		assert.Equal(t, int64(-4211780967), result.GroupID)
		assert.Equal(t, []string{"a", "1000", "30m"}, remainingArgs)
	})

	t.Run("private chat with no args", func(t *testing.T) {
		privateChatID := int64(290924591)
		args := []string{}

		result, remainingArgs, err := bot.ResolveTargetGroup(privateChatID, args)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "specify the target group")
		assert.Nil(t, result)
		assert.Nil(t, remainingArgs)
	})

	t.Run("group chat uses chatID directly", func(t *testing.T) {
		groupChatID := int64(-4211780967) // Negative = group chat
		args := []string{"a", "1000", "30m"}

		result, remainingArgs, err := bot.ResolveTargetGroup(groupChatID, args)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.IsName)
		assert.Equal(t, groupChatID, result.GroupID)
		assert.Equal(t, args, remainingArgs, "Args should be unchanged for group chat")
	})

	t.Run("group chat ignores @username in args", func(t *testing.T) {
		groupChatID := int64(-4211780967)
		args := []string{"@home", "a", "1000", "30m"} // @home is just treated as first arg

		result, remainingArgs, err := bot.ResolveTargetGroup(groupChatID, args)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, groupChatID, result.GroupID)
		assert.Equal(t, args, remainingArgs, "All args should be preserved for group chat")
	})
}

// TestResolveGroupID tests username resolution to numeric ID
func TestResolveGroupID(t *testing.T) {
	t.Run("numeric identifier returns directly", func(t *testing.T) {
		identifier := &bot.GroupIdentifier{
			GroupID: -4211780967,
			IsName:  false,
		}

		// Resolver shouldn't be called
		resolver := &mockGroupResolver{
			resolveUsernameFunc: func(username string) (int64, error) {
				t.Fatal("Resolver should not be called for numeric ID")
				return 0, nil
			},
		}

		groupID, err := bot.ResolveGroupID(identifier, resolver)

		assert.NoError(t, err)
		assert.Equal(t, int64(-4211780967), groupID)
	})

	t.Run("username identifier calls resolver", func(t *testing.T) {
		identifier := &bot.GroupIdentifier{
			Username: "home",
			IsName:   true,
		}

		resolverCalled := false
		resolver := &mockGroupResolver{
			resolveUsernameFunc: func(username string) (int64, error) {
				resolverCalled = true
				assert.Equal(t, "home", username)
				return -4211780967, nil
			},
		}

		groupID, err := bot.ResolveGroupID(identifier, resolver)

		assert.NoError(t, err)
		assert.True(t, resolverCalled, "Resolver should be called for username")
		assert.Equal(t, int64(-4211780967), groupID)
	})

	t.Run("resolver error propagates", func(t *testing.T) {
		identifier := &bot.GroupIdentifier{
			Username: "nonexistent",
			IsName:   true,
		}

		resolver := &mockGroupResolver{
			resolveUsernameFunc: func(username string) (int64, error) {
				return 0, assert.AnError
			},
		}

		groupID, err := bot.ResolveGroupID(identifier, resolver)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to resolve @nonexistent")
		assert.Equal(t, int64(0), groupID)
	})
}
