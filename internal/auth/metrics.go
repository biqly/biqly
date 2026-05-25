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

	MetricDatasourceAccessChecks = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_datasource_access_checks_total",
			Help: "Total number of datasource access checks",
		},
		[]string{"result"},
	)
)
