package metrics

import (
	"testing"
	"time"
)

func TestRecordCommand(_ *testing.T) {
	// Test that RecordCommand doesn't panic
	RecordCommand("test", "private", 100*time.Millisecond)
	RecordCommand("help", "group", 50*time.Millisecond)
}

func TestRecordError(_ *testing.T) {
	// Test that RecordError doesn't panic
	RecordError("test_error", "test_operation")
	RecordError("database_error", "query")
}

func TestRecordRateLimitEnforcement(_ *testing.T) {
	// Test that RecordRateLimitEnforcement doesn't panic
	RecordRateLimitEnforcement("restrict", "12345")
	RecordRateLimitEnforcement("delete_message", "67890")
}

func TestRecordRateLimitCheck(_ *testing.T) {
	// Test that RecordRateLimitCheck doesn't panic
	RecordRateLimitCheck("allowed")
	RecordRateLimitCheck("exceeded")
	RecordRateLimitCheck("exempt")
}

func TestRecordDatabaseQuery(_ *testing.T) {
	// Test that RecordDatabaseQuery doesn't panic
	RecordDatabaseQuery("GetUserCurrentUsage", 10*time.Millisecond)
	RecordDatabaseQuery("StoreMessage", 5*time.Millisecond)
}

func TestRecordMessage(_ *testing.T) {
	// Test that RecordMessage doesn't panic
	RecordMessage("private")
	RecordMessage("group")
	RecordMessage("supergroup")
}

func TestSetActiveRestrictions(_ *testing.T) {
	// Test that SetActiveRestrictions doesn't panic
	SetActiveRestrictions(5)
	SetActiveRestrictions(0)
	SetActiveRestrictions(100)
}

func TestAddCharactersTracked(_ *testing.T) {
	// Test that AddCharactersTracked doesn't panic
	AddCharactersTracked(10)
	AddCharactersTracked(100)
	AddCharactersTracked(1000)
}

func TestTimer(_ *testing.T) {
	// Test Timer functionality
	timer := NewTimer()
	time.Sleep(10 * time.Millisecond)

	// Should not panic
	timer.ObserveDuration("test_operation")
}
