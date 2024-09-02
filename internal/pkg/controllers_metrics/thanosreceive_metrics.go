package controllers_metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type ThanosReceiveMetrics struct {
	*BaseMetrics
	HashringsConfigured                 *prometheus.GaugeVec
	EndpointWatchesReconciliationsTotal prometheus.Counter
}

func NewThanosReceiveMetrics(reg prometheus.Registerer) ThanosReceiveMetrics {
	return ThanosReceiveMetrics{
		BaseMetrics: NewBaseMetrics(reg, "receive"),
		HashringsConfigured: promauto.With(reg).NewGaugeVec(prometheus.GaugeOpts{
			Name: "thanos_operator_receive_hashrings_configured",
			Help: "Number of configured hashrings for ThanosReceive resources",
		}, []string{"name", "namespace"}),
		EndpointWatchesReconciliationsTotal: promauto.With(reg).NewCounter(prometheus.CounterOpts{
			Name: "thanos_operator_receive_endpoint_event_reconciliations_total",
			Help: "Total number of reconciliations for ThanosReceive resources due to EndpointSlice events",
		}),
	}
}
