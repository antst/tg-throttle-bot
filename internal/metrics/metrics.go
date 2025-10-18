// Package metrics provides Prometheus instrumentation for performance monitoring.
// It tracks command execution, errors, rate limit enforcement, and database query times.
package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// CommandsTotal tracks the total number of commands processed by type
	CommandsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "throttlebot_commands_total",
			Help: "Total number of commands processed by the bot",
		},
		[]string{"command", "chat_type"},
	)

	// CommandDuration tracks command execution time
	CommandDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "throttlebot_command_duration_seconds",
			Help:    "Command execution duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"command"},
	)

	// ErrorsTotal tracks errors by type
	ErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "throttlebot_errors_total",
			Help: "Total number of errors by type",
		},
		[]string{"error_type", "operation"},
	)

	// RateLimitEnforcements tracks rate limit enforcement actions
	RateLimitEnforcements = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "throttlebot_ratelimit_enforcements_total",
			Help: "Total number of rate limit enforcements",
		},
		[]string{"action", "chat_id"},
	)

	// RateLimitChecks tracks rate limit checks
	RateLimitChecks = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "throttlebot_ratelimit_checks_total",
			Help: "Total number of rate limit checks",
		},
		[]string{"result"},
	)

	// DatabaseQueryDuration tracks database query execution time
	DatabaseQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "throttlebot_database_query_duration_seconds",
			Help:    "Database query execution duration in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"operation"},
	)

	// MessagesProcessed tracks total messages processed
	MessagesProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "throttlebot_messages_processed_total",
			Help: "Total number of messages processed",
		},
		[]string{"chat_type"},
	)

	// ActiveRestrictions tracks currently restricted users
	ActiveRestrictions = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "throttlebot_active_restrictions",
			Help: "Number of currently restricted users",
		},
	)

	// CharactersTracked tracks total characters processed
	CharactersTracked = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "throttlebot_characters_tracked_total",
			Help: "Total number of characters tracked for rate limiting",
		},
	)

	// ========================================================================
	// Multi-Window Rate Limiting Metrics (Feature 006)
	// ========================================================================

	// WindowViolations tracks violations per window slot
	WindowViolations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "throttlebot_window_violations_total",
			Help: "Total number of rate limit violations per window slot",
		},
		[]string{"chat_id", "window_slot"},
	)

	// WindowUsagePercent tracks current usage percentage per window
	WindowUsagePercent = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "throttlebot_window_usage_percent",
			Help: "Current usage percentage per window slot",
		},
		[]string{"chat_id", "window_slot"},
	)

	// WindowEnforcementDuration tracks time to evaluate a single window
	WindowEnforcementDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "throttlebot_window_enforcement_duration_seconds",
			Help:    "Time to evaluate a single window for rate limiting",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"window_slot"},
	)

	// WindowConfigChanges tracks window configuration changes
	WindowConfigChanges = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "throttlebot_window_config_changes_total",
			Help: "Total number of window configuration changes",
		},
		[]string{"chat_id", "window_slot", "action"},
	)

	// WindowWarnings tracks warnings sent per window
	WindowWarnings = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "throttlebot_window_warnings_total",
			Help: "Total number of warnings sent per window threshold",
		},
		[]string{"chat_id", "window_slot", "threshold"},
	)

	// MultiWindowEvaluationDuration tracks total time to evaluate all windows
	MultiWindowEvaluationDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "throttlebot_multi_window_evaluation_duration_seconds",
			Help:    "Total time to evaluate all enabled windows for a message",
			Buckets: []float64{.01, .025, .05, .1, .25, .5, 1, 2},
		},
	)
)

// RecordCommand records a command execution with its duration
func RecordCommand(command string, chatType string, duration time.Duration) {
	CommandsTotal.WithLabelValues(command, chatType).Inc()
	CommandDuration.WithLabelValues(command).Observe(duration.Seconds())
}

// RecordError records an error occurrence
func RecordError(errorType string, operation string) {
	ErrorsTotal.WithLabelValues(errorType, operation).Inc()
}

// RecordRateLimitEnforcement records a rate limit enforcement action
func RecordRateLimitEnforcement(action string, chatID string) {
	RateLimitEnforcements.WithLabelValues(action, chatID).Inc()
}

// RecordRateLimitCheck records a rate limit check result
func RecordRateLimitCheck(result string) {
	RateLimitChecks.WithLabelValues(result).Inc()
}

// RecordDatabaseQuery records a database query duration
func RecordDatabaseQuery(operation string, duration time.Duration) {
	DatabaseQueryDuration.WithLabelValues(operation).Observe(duration.Seconds())
}

// RecordMessage records a processed message
func RecordMessage(chatType string) {
	MessagesProcessed.WithLabelValues(chatType).Inc()
}

// SetActiveRestrictions sets the gauge for currently active restrictions
func SetActiveRestrictions(count int) {
	ActiveRestrictions.Set(float64(count))
}

// AddCharactersTracked adds to the total characters tracked
func AddCharactersTracked(count int) {
	CharactersTracked.Add(float64(count))
}

// ============================================================================
// Multi-Window Metrics Functions (Feature 006)
// ============================================================================

// RecordWindowViolation records a violation for a specific window
func RecordWindowViolation(chatID string, windowSlot string) {
	WindowViolations.WithLabelValues(chatID, windowSlot).Inc()
}

// SetWindowUsage sets the current usage percentage for a window
func SetWindowUsage(chatID string, windowSlot string, usagePercent float64) {
	WindowUsagePercent.WithLabelValues(chatID, windowSlot).Set(usagePercent)
}

// RecordWindowEnforcement records the time to evaluate a single window
func RecordWindowEnforcement(windowSlot string, duration time.Duration) {
	WindowEnforcementDuration.WithLabelValues(windowSlot).Observe(duration.Seconds())
}

// RecordWindowConfigChange records a window configuration change
func RecordWindowConfigChange(chatID string, windowSlot string, action string) {
	WindowConfigChanges.WithLabelValues(chatID, windowSlot, action).Inc()
}

// RecordWindowWarning records a warning sent for a window threshold
func RecordWindowWarning(chatID string, windowSlot string, threshold string) {
	WindowWarnings.WithLabelValues(chatID, windowSlot, threshold).Inc()
}

// RecordMultiWindowEvaluation records the total time to evaluate all windows
func RecordMultiWindowEvaluation(duration time.Duration) {
	MultiWindowEvaluationDuration.Observe(duration.Seconds())
}

// Timer is a helper for timing operations
type Timer struct {
	start time.Time
}

// NewTimer creates a new timer
func NewTimer() *Timer {
	return &Timer{start: time.Now()}
}

// ObserveDuration records the duration since timer creation
func (t *Timer) ObserveDuration(operation string) {
	duration := time.Since(t.start)
	RecordDatabaseQuery(operation, duration)
}
