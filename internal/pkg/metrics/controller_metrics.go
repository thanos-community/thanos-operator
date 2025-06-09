package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type CommonMetrics struct {
	FeatureGatesEnabled *prometheus.GaugeVec
	Paused              *prometheus.GaugeVec
	ResourceSync        *prometheus.CounterVec
}

type ThanosQueryMetrics struct {
	*CommonMetrics
	EndpointsConfigured                *prometheus.GaugeVec
	ServiceWatchesReconciliationsTotal *prometheus.CounterVec
}

type ThanosReceiveMetrics struct {
	*CommonMetrics
	HashringsConfigured                 *prometheus.GaugeVec
	HashringHash                        *prometheus.GaugeVec
	HashringTenantsConfigured           *prometheus.GaugeVec
	HashringEndpointsConfigured         *prometheus.GaugeVec
	EndpointWatchesReconciliationsTotal *prometheus.CounterVec
}

type ThanosRulerMetrics struct {
	*CommonMetrics
	EndpointsConfigured                       *prometheus.GaugeVec
	RuleFilesConfigured                       *prometheus.GaugeVec
	PrometheusRulesFound                      *prometheus.GaugeVec
	PrometheusRuleGroupsFound                 *prometheus.GaugeVec
	PrometheusRuleGroupsTenantCount           *prometheus.GaugeVec
	ConfigMapsCreated                         *prometheus.CounterVec
	ConfigMapCreationFailures                 *prometheus.CounterVec
	ServiceWatchesReconciliationsTotal        *prometheus.CounterVec
	ConfigMapWatchesReconciliationsTotal      *prometheus.CounterVec
	PrometheusRuleWatchesReconciliationsTotal *prometheus.CounterVec
}

type ThanosStoreMetrics struct {
	*CommonMetrics
	ShardsConfigured            *prometheus.GaugeVec
	ShardCreationUpdateFailures *prometheus.CounterVec
}

type ThanosCompactMetrics struct {
	*CommonMetrics
	ShardsConfigured            *prometheus.GaugeVec
	ShardCreationUpdateFailures *prometheus.CounterVec
}

var (
	commonMetricsInstance *CommonMetrics
	commonMetricsOnce     sync.Once
)

func NewCommonMetrics(reg prometheus.Registerer) *CommonMetrics {
	commonMetricsOnce.Do(func() {
		commonMetricsInstance = &CommonMetrics{
			FeatureGatesEnabled: promauto.With(reg).NewGaugeVec(prometheus.GaugeOpts{
				Name: "thanos_operator_feature_gates_enabled",
				Help: "Feature gates enabled for ThanosOperator",
			}, []string{"component"}),
			Paused: promauto.With(reg).NewGaugeVec(prometheus.GaugeOpts{
				Name: "thanos_operator_paused",
				Help: "Paused state of ThanosOperator",
			}, []string{"component", "resource", "namespace"}),
			ResourceSync: promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
				Name: "thanos_operator_resource_sync",
				Help: "Total number of resources synced by ThanosOperator",
			}, []string{"component", "resource", "namespace", "case"}),
		}
	})
	return commonMetricsInstance
}

func NewThanosQueryMetrics(reg prometheus.Registerer, commonMetrics *CommonMetrics) ThanosQueryMetrics {
	return ThanosQueryMetrics{
		CommonMetrics: commonMetrics,
		EndpointsConfigured: promauto.With(reg).NewGaugeVec(prometheus.GaugeOpts{
			Name: "thanos_operator_query_endpoints_configured",
			Help: "Number of configured endpoints for ThanosQuery resources",
		}, []string{"type", "resource", "namespace"}),
		ServiceWatchesReconciliationsTotal: promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
			Name: "thanos_operator_query_service_event_reconciliations_total",
			Help: "Total number of reconciliations for ThanosQuery resources due to Service events",
		}, []string{"resource", "namespace"}),
	}
}

func NewThanosReceiveMetrics(reg prometheus.Registerer, commonMetrics *CommonMetrics) ThanosReceiveMetrics {
	return ThanosReceiveMetrics{
		CommonMetrics: commonMetrics,
		HashringsConfigured: promauto.With(reg).NewGaugeVec(prometheus.GaugeOpts{
			Name: "thanos_operator_receive_hashrings_configured",
			Help: "Number of configured hashrings for ThanosReceive resources",
		}, []string{"resource", "namespace"}),
		HashringHash: promauto.With(reg).NewGaugeVec(prometheus.GaugeOpts{
			Name: "thanos_operator_receive_hashring_hash",
			Help: "Hash of the hashrings configuration for ThanosReceive resources",
		}, []string{"resource", "namespace"}),
		HashringTenantsConfigured: promauto.With(reg).NewGaugeVec(prometheus.GaugeOpts{
			Name: "thanos_operator_receive_hashring_tenants_configured",
			Help: "Number of configured tenants for hashrings for ThanosReceive resources",
		}, []string{"resource", "namespace", "hashring"}),
		HashringEndpointsConfigured: promauto.With(reg).NewGaugeVec(prometheus.GaugeOpts{
			Name: "thanos_operator_receive_hashring_endpoints_configured",
			Help: "Number of configured endpoints for hashrings for ThanosReceive resources",
		}, []string{"resource", "namespace", "hashring"}),
		EndpointWatchesReconciliationsTotal: promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
			Name: "thanos_operator_receive_endpoint_event_reconciliations_total",
			Help: "Total number of reconciliations for ThanosReceive resources due to EndpointSlice events",
		}, []string{"resource", "namespace"}),
	}
}

func NewThanosRulerMetrics(reg prometheus.Registerer, commonMetrics *CommonMetrics) ThanosRulerMetrics {
	return ThanosRulerMetrics{
		CommonMetrics: commonMetrics,
		EndpointsConfigured: promauto.With(reg).NewGaugeVec(prometheus.GaugeOpts{
			Name: "thanos_operator_ruler_query_endpoints_configured",
			Help: "Number of configured query endpoints for ThanosRuler resources",
		}, []string{"resource", "namespace"}),
		RuleFilesConfigured: promauto.With(reg).NewGaugeVec(prometheus.GaugeOpts{
			Name: "thanos_operator_ruler_rulefiles_configured",
			Help: "Number of configured rulefiles for ThanosRuler resources",
		}, []string{"resource", "namespace"}),
		ServiceWatchesReconciliationsTotal: promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
			Name: "thanos_operator_ruler_service_event_reconciliations_total",
			Help: "Total number of reconciliations for ThanosRuler resources due to Service events",
		}, []string{"resource", "namespace"}),
		ConfigMapWatchesReconciliationsTotal: promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
			Name: "thanos_operator_ruler_cfgmap_event_reconciliations_total",
			Help: "Total number of reconciliations for ThanosRuler resources due to ConfigMap events",
		}, []string{"resource", "namespace"}),
		PrometheusRuleWatchesReconciliationsTotal: promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
			Name: "thanos_operator_ruler_promrule_event_reconciliations_total",
			Help: "Total number of reconciliations for ThanosRuler resources due to PrometheusRule events",
		}, []string{"resource", "namespace"}),
		PrometheusRulesFound: promauto.With(reg).NewGaugeVec(prometheus.GaugeOpts{
			Name: "thanos_operator_ruler_promrules_found",
			Help: "Number of PrometheusRules found for ThanosRuler resources",
		}, []string{"resource", "namespace"}),
		PrometheusRuleGroupsFound: promauto.With(reg).NewGaugeVec(prometheus.GaugeOpts{
			Name: "thanos_operator_ruler_promrule_groups_found",
			Help: "Number of PrometheusRule groups found for ThanosRuler resources",
		}, []string{"resource", "namespace", "rule_name"}),
		ConfigMapsCreated: promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
			Name: "thanos_operator_ruler_cfgmaps_created",
			Help: "Number of ConfigMaps created from PrometheusRules for ThanosRuler resources",
		}, []string{"resource", "namespace"}),
		ConfigMapCreationFailures: promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
			Name: "thanos_operator_ruler_cfgmaps_creation_failures",
			Help: "Number of ConfigMaps creation failures for ThanosRuler resources",
		}, []string{"resource", "namespace"}),
		PrometheusRuleGroupsTenantCount: promauto.With(reg).NewGaugeVec(prometheus.GaugeOpts{
			Name: "thanos_operator_ruler_promrule_groups_tenant_count",
			Help: "Number of PrometheusRule groups found for ThanosRuler resources by tenant",
		}, []string{"resource", "namespace", "tenant"}),
	}
}

func NewThanosStoreMetrics(reg prometheus.Registerer, commonMetrics *CommonMetrics) ThanosStoreMetrics {
	return ThanosStoreMetrics{
		CommonMetrics: commonMetrics,
		ShardsConfigured: promauto.With(reg).NewGaugeVec(prometheus.GaugeOpts{
			Name: "thanos_operator_store_shards_configured",
			Help: "Number of shards configured for ThanosStore resources",
		}, []string{"resource", "namespace"}),
		ShardCreationUpdateFailures: promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
			Name: "thanos_operator_store_shards_creation_update_failures",
			Help: "Number of shard creation/update failures for ThanosStore resources",
		}, []string{"resource", "namespace"}),
	}
}

func NewThanosCompactMetrics(reg prometheus.Registerer, commonMetrics *CommonMetrics) ThanosCompactMetrics {
	return ThanosCompactMetrics{
		CommonMetrics: commonMetrics,
		ShardsConfigured: promauto.With(reg).NewGaugeVec(prometheus.GaugeOpts{
			Name: "thanos_operator_compact_shards_configured",
			Help: "Number of shards configured for ThanosCompact resources",
		}, []string{"resource", "namespace"}),
		ShardCreationUpdateFailures: promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
			Name: "thanos_operator_compact_shards_creation_update_failures",
			Help: "Number of shard creation/update failures for ThanosCompact resources",
		}, []string{"resource", "namespace"}),
	}
}
