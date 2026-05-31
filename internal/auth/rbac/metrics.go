package rbac

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	MetricDatasourceAccessChecks = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_datasource_access_checks_total",
			Help: "Total number of datasource access checks",
		},
		[]string{"result"},
	)
)
