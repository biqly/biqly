package auth

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	MetricLoginAttempts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_login_attempts_total",
			Help: "Total number of login attempts",
		},
		[]string{"method", "status"},
	)

	MetricTokenRefreshes = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_token_refreshes_total",
			Help: "Total number of token refreshes",
		},
		[]string{"status"},
	)

	// MetricFailedLogins breaks failed logins down by reason so alerts can
	// distinguish brute-force (bad_password) from enumeration probes
	// (user_not_found) and account-state rejections.
	MetricFailedLogins = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_failed_login_total",
			Help: "Total number of failed logins by reason",
		},
		[]string{"reason"},
	)

	// MetricTokensIssued counts access tokens minted at the end of a
	// successful authentication, labelled by the method that produced them.
	MetricTokensIssued = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_token_issued_total",
			Help: "Total number of access tokens issued",
		},
		[]string{"method"},
	)

	// MetricPermissionCheckDuration tracks the latency of internal permission
	// checks, the hot path for every authorized monolith request.
	MetricPermissionCheckDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "auth_permission_check_duration_seconds",
			Help:    "Duration of internal permission checks",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"result"},
	)
)

// Failed-login reason labels for MetricFailedLogins. Kept as constants so the
// label cardinality stays bounded and greppable.
const (
	LoginFailAccountLocked  = "account_locked"
	LoginFailUserNotFound   = "user_not_found"
	LoginFailInactive       = "inactive"
	LoginFailBadPassword    = "bad_password"
	LoginFailAccountDeleted = "account_deleted"
	LoginFailAccountFrozen  = "account_frozen"
	LoginFailMFAInvalid     = "mfa_invalid"
)
