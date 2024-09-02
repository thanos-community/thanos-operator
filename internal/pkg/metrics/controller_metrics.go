package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type BaseMetrics struct {
	ReconciliationsTotal       *prometheus.CounterVec
	ReconciliationsFailedTotal *prometheus.CounterVec
	ClientErrorsTotal          *prometheus.CounterVec
}

type ThanosQueryMetrics struct {
	EndpointsConfigured                *prometheus.GaugeVec
	ServiceWatchesReconciliationsTotal prometheus.Counter
}

type ThanosReceiveMetrics struct {
	HashringsConfigured                 *prometheus.GaugeVec
	EndpointWatchesReconciliationsTotal prometheus.Counter
}

func NewBaseMetrics(reg prometheus.Registerer) *BaseMetrics {
	return &BaseMetrics{
		ReconciliationsTotal: promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
			Name: "thanos_operator_reconciliations_total",
			Help: "Total number of reconciliations for Thanos resources",
		}, []string{"component"}),
		ReconciliationsFailedTotal: promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
			Name: "thanos_operator_reconciliations_failed_total",
			Help: "Total number of failed reconciliations for Thanos resources",
		}, []string{"component"}),
		ClientErrorsTotal: promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
			Name: "thanos_operator_client_errors_total",
			Help: "Total number of errors encountered during kube client calls of Thanos resources",
		}, []string{"component"}),
	}
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

func NewThanosReceiveMetrics(reg prometheus.Registerer, controlllerBasemetrics *BaseMetrics) ThanosReceiveMetrics {
	return ThanosReceiveMetrics{
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
