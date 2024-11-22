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
	*BaseMetrics
	EndpointsConfigured                        *prometheus.GaugeVec
	ServiceWatchesReconciliationsTotal         prometheus.Counter
	FrontendServiceWatchesReconciliationsTotal prometheus.Counter
}

type ThanosReceiveMetrics struct {
	HashringsConfigured                 *prometheus.GaugeVec
	EndpointWatchesReconciliationsTotal prometheus.Counter
	HashringHash                        *prometheus.GaugeVec
}

type ThanosRulerMetrics struct {
	*BaseMetrics
	EndpointsConfigured                  *prometheus.GaugeVec
	RuleFilesConfigured                  *prometheus.GaugeVec
	ServiceWatchesReconciliationsTotal   prometheus.Counter
	ConfigMapWatchesReconciliationsTotal prometheus.Counter
}

type ThanosStoreMetrics struct {
	*BaseMetrics
}

type ThanosCompactMetrics struct {
	*BaseMetrics
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

func NewThanosQueryMetrics(reg prometheus.Registerer, baseMetrics *BaseMetrics) ThanosQueryMetrics {
	return ThanosQueryMetrics{
		BaseMetrics: baseMetrics,
		EndpointsConfigured: promauto.With(reg).NewGaugeVec(prometheus.GaugeOpts{
			Name: "thanos_operator_query_endpoints_configured",
			Help: "Number of configured endpoints for ThanosQuery resources",
		}, []string{"type", "resource", "namespace"}),
		ServiceWatchesReconciliationsTotal: promauto.With(reg).NewCounter(prometheus.CounterOpts{
			Name: "thanos_operator_query_service_event_reconciliations_total",
			Help: "Total number of reconciliations for ThanosQuery resources due to Service events",
		}),
		FrontendServiceWatchesReconciliationsTotal: promauto.With(reg).NewCounter(prometheus.CounterOpts{
			Name: "thanos_operator_query_frontend_service_event_reconciliations_total",
			Help: "Total number of reconciliations for ThanosQueryFrontend resources due to Service events",
		}),
	}
}

func NewThanosReceiveMetrics(reg prometheus.Registerer) ThanosReceiveMetrics {
	return ThanosReceiveMetrics{
		HashringsConfigured: promauto.With(reg).NewGaugeVec(prometheus.GaugeOpts{
			Name: "thanos_operator_receive_hashrings_configured",
			Help: "Number of configured hashrings for ThanosReceive resources",
		}, []string{"resource", "namespace"}),
		HashringHash: promauto.With(reg).NewGaugeVec(prometheus.GaugeOpts{
			Name: "thanos_operator_receive_hashring_hash",
			Help: "Hash of the hashring for ThanosReceive resources",
		}, []string{"resource", "namespace"}),
		EndpointWatchesReconciliationsTotal: promauto.With(reg).NewCounter(prometheus.CounterOpts{
			Name: "thanos_operator_receive_endpoint_event_reconciliations_total",
			Help: "Total number of reconciliations for ThanosReceive resources due to EndpointSlice events",
		}),
	}
}

func NewThanosRulerMetrics(reg prometheus.Registerer, baseMetrics *BaseMetrics) ThanosRulerMetrics {
	return ThanosRulerMetrics{
		BaseMetrics: baseMetrics,
		EndpointsConfigured: promauto.With(reg).NewGaugeVec(prometheus.GaugeOpts{
			Name: "thanos_operator_ruler_query_endpoints_configured",
			Help: "Number of configured query endpoints for ThanosRuler resources",
		}, []string{"resource", "namespace"}),
		RuleFilesConfigured: promauto.With(reg).NewGaugeVec(prometheus.GaugeOpts{
			Name: "thanos_operator_ruler_rulefiles_configured",
			Help: "Number of configured rulefiles for ThanosRuler resources",
		}, []string{"resource", "namespace"}),
		ServiceWatchesReconciliationsTotal: promauto.With(reg).NewCounter(prometheus.CounterOpts{
			Name: "thanos_operator_ruler_service_event_reconciliations_total",
			Help: "Total number of reconciliations for ThanosRuler resources due to Service events",
		}),
		ConfigMapWatchesReconciliationsTotal: promauto.With(reg).NewCounter(prometheus.CounterOpts{
			Name: "thanos_operator_ruler_cfgmap_event_reconciliations_total",
			Help: "Total number of reconciliations for ThanosRuler resources due to ConfigMap events",
		}),
	}
}

func NewThanosStoreMetrics(reg prometheus.Registerer, baseMetrics *BaseMetrics) ThanosStoreMetrics {
	return ThanosStoreMetrics{
		BaseMetrics: baseMetrics,
	}
}

func NewThanosCompactMetrics(reg prometheus.Registerer, baseMetrics *BaseMetrics) ThanosCompactMetrics {
	return ThanosCompactMetrics{
		BaseMetrics: baseMetrics,
	}
}
