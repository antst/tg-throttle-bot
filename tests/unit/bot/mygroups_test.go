// Package bot_test contains unit tests for bot command handlers
package bot_test

import (
	"testing"

	"github.com/antst/tg-throttle-bot/internal/bot"
	"github.com/antst/tg-throttle-bot/internal/i18n"
	"github.com/antst/tg-throttle-bot/internal/ratelimit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain initializes i18n for all tests
func TestMain(m *testing.M) {
	// Initialize i18n bundle
	if err := i18n.Init(); err != nil {
		panic(err)
	}
	m.Run()
}

// TestParseMyGroupsArgs tests the page number parsing logic
func TestParseMyGroupsArgs(t *testing.T) {
	t.Run("no arguments returns page 1", func(t *testing.T) {
		page, err := bot.ParseMyGroupsArgs([]string{})
		require.NoError(t, err)
		assert.Equal(t, 1, page)
	})

	t.Run("valid page number parsed correctly", func(t *testing.T) {
		page, err := bot.ParseMyGroupsArgs([]string{"2"})
		require.NoError(t, err)
		assert.Equal(t, 2, page)
	})

	t.Run("invalid page number returns error", func(t *testing.T) {
		_, err := bot.ParseMyGroupsArgs([]string{"abc"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid page number")
	})

	t.Run("negative page number returns error", func(t *testing.T) {
		_, err := bot.ParseMyGroupsArgs([]string{"-1"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be >= 1")
	})

	t.Run("zero page number returns error", func(t *testing.T) {
		_, err := bot.ParseMyGroupsArgs([]string{"0"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be >= 1")
	})
}

// TestFormatGroupList tests formatting of group list with various scenarios
func TestFormatGroupList(t *testing.T) {
	cmdCtx := bot.CommandContext{
		UserLanguage: "en",
		UserID:       123456,
		ChatID:       123456,
		ChatType:     "private",
	}

	t.Run("single page with no pagination indicator", func(t *testing.T) {
		title := "Test Group"
		groups := []ratelimit.UserGroupMembership{
			{ChatID: -100, Title: &title, IsAdmin: true},
		}
		result := bot.FormatGroupList(cmdCtx, groups, 1, 1)
		assert.Contains(t, result, "Test Group")
		assert.Contains(t, result, "admin")
		assert.NotContains(t, result, "Use /mygroups")
	})

	t.Run("multiple pages shows pagination", func(t *testing.T) {
		groups := make([]ratelimit.UserGroupMembership, 20) // Full page
		for i := range groups {
			title := "Group"
			groups[i] = ratelimit.UserGroupMembership{ChatID: int64(-100 - i), Title: &title, IsAdmin: false}
		}
		result := bot.FormatGroupList(cmdCtx, groups, 1, 3)
		assert.Contains(t, result, "Page 1 of 3")
		assert.Contains(t, result, "Use /mygroups 2")
	})

	t.Run("empty state shows no groups message", func(t *testing.T) {
		groups := []ratelimit.UserGroupMembership{}
		result := bot.FormatGroupList(cmdCtx, groups, 1, 1)
		assert.Contains(t, result, "not a member of any groups")
	})

	t.Run("NULL group title shows fallback text", func(t *testing.T) {
		groups := []ratelimit.UserGroupMembership{
			{ChatID: -100, Title: nil, IsAdmin: false},
		}
		result := bot.FormatGroupList(cmdCtx, groups, 1, 1)
		assert.Contains(t, result, "Unnamed Group (ID: -100)")
		assert.Contains(t, result, "user")
	})

	t.Run("distinguishes admin vs regular user roles", func(t *testing.T) {
		adminTitle := "Admin Group"
		userTitle := "User Group"
		groups := []ratelimit.UserGroupMembership{
			{ChatID: -100, Title: &adminTitle, IsAdmin: true},
			{ChatID: -101, Title: &userTitle, IsAdmin: false},
		}
		result := bot.FormatGroupList(cmdCtx, groups, 1, 1)
		assert.Contains(t, result, "Admin Group")
		assert.Contains(t, result, "admin")
		assert.Contains(t, result, "User Group")
		assert.Contains(t, result, "user")
	})
}

// TestCalculatePagination tests pagination calculation edge cases
func TestCalculatePagination(t *testing.T) {
	t.Run("zero groups returns single page", func(t *testing.T) {
		total, current, offset := bot.CalculatePagination(0, 1, 20)
		assert.Equal(t, 1, total)
		assert.Equal(t, 1, current)
		assert.Equal(t, 0, offset)
	})

	t.Run("exactly 20 groups is one page", func(t *testing.T) {
		total, _, _ := bot.CalculatePagination(20, 1, 20)
		assert.Equal(t, 1, total)
	})

	t.Run("21 groups creates 2 pages", func(t *testing.T) {
		total, _, _ := bot.CalculatePagination(21, 1, 20)
		assert.Equal(t, 2, total)
	})

	t.Run("page number exceeding total pages clamps to last page", func(t *testing.T) {
		total, current, _ := bot.CalculatePagination(25, 10, 20)
		assert.Equal(t, 2, total)
		assert.Equal(t, 2, current) // Clamped to last page
	})

	t.Run("calculates correct offset for page 2", func(t *testing.T) {
		_, _, offset := bot.CalculatePagination(40, 2, 20)
		assert.Equal(t, 20, offset)
	})
}
