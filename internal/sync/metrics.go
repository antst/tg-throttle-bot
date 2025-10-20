// Package sync provides proactive member synchronization for Telegram groups.
package sync

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// MemberSyncTotal tracks the total number of sync operations.
	// Labels: type (initial_sync|periodic_sync|notification_processed), status (success|failure)
	MemberSyncTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "member_sync_total",
			Help: "Total number of member sync operations",
		},
		[]string{"type", "status"},
	)

	// MemberSyncFallbackTotal tracks reactive fallback updates.
	// Incremented when a message arrives from a user not in the database.
	MemberSyncFallbackTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "member_sync_fallback_total",
			Help: "Total number of reactive fallback member updates",
		},
	)

	// MemberSyncNotificationDuration tracks notification processing latency.
	// Labels: notification_type (my_chat_member|chat_member)
	MemberSyncNotificationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "member_sync_notification_duration_seconds",
			Help:    "Time to process member notification",
			Buckets: prometheus.DefBuckets, // includes p50, p95, p99
		},
		[]string{"notification_type"},
	)

	// MemberSyncFailures tracks failed sync operations.
	// Labels: type (initial_sync|periodic_sync), reason (timeout|rate_limit|permission|other)
	MemberSyncFailures = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "member_sync_failures_total",
			Help: "Total number of failed member sync operations",
		},
		[]string{"type", "reason"},
	)

	// MemberSyncStaleRecords tracks stale membership records (updated_at > 48h).
	// Labels: chat_id
	MemberSyncStaleRecords = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "member_sync_stale_records",
			Help: "Number of stale membership records (updated_at > 48h)",
		},
		[]string{"chat_id"},
	)
)

// Metric label constants for consistency
const (
	// Sync types
	SyncTypeInitial      = "initial_sync"
	SyncTypePeriodic     = "periodic_sync"
	SyncTypeNotification = "notification_processed"

	// Sync statuses
	SyncStatusSuccess = "success"
	SyncStatusFailure = "failure"

	// Notification types
	NotificationTypeMyChatMember = "my_chat_member"
	NotificationTypeChatMember   = "chat_member"

	// Failure reasons
	FailureReasonTimeout    = "timeout"
	FailureReasonRateLimit  = "rate_limit"
	FailureReasonPermission = "permission"
	FailureReasonOther      = "other"
)
