package controllers_metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type ThanosQueryMetrics struct {
	EndpointsConfigured                *prometheus.GaugeVec
	ServiceWatchesReconciliationsTotal prometheus.Counter
}

func NewThanosQueryMetrics(reg prometheus.Registerer) ThanosQueryMetrics {
	return ThanosQueryMetrics{
		EndpointsConfigured: promauto.With(reg).NewGaugeVec(prometheus.GaugeOpts{
			Name: "thanos_operator_query_endpoints_configured",
			Help: "Number of configured endpoints for ThanosQuery resources",
		}, []string{"type", "resource", "namespace"}),
		ServiceWatchesReconciliationsTotal: promauto.With(reg).NewCounter(prometheus.CounterOpts{
			Name: "thanos_operator_query_service_event_reconciliations_total",
			Help: "Total number of reconciliations for ThanosQuery resources due to Service events",
		}),
	}
}
