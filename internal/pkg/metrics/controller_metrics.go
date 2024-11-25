package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type ThanosQueryMetrics struct {
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
	EndpointsConfigured                  *prometheus.GaugeVec
	RuleFilesConfigured                  *prometheus.GaugeVec
	ServiceWatchesReconciliationsTotal   prometheus.Counter
	ConfigMapWatchesReconciliationsTotal prometheus.Counter
}

type ThanosStoreMetrics struct {
}

type ThanosCompactMetrics struct {
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

func NewThanosRulerMetrics(reg prometheus.Registerer) ThanosRulerMetrics {
	return ThanosRulerMetrics{
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

func NewThanosStoreMetrics(reg prometheus.Registerer) ThanosStoreMetrics {
	return ThanosStoreMetrics{}
}

func NewThanosCompactMetrics(reg prometheus.Registerer) ThanosCompactMetrics {
	return ThanosCompactMetrics{}
}
