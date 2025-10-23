package controller

import (
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"

	"github.com/thanos-community/thanos-operator/internal/pkg/metrics"
)

// Config holds the configuration for all controllers.
type Config struct {
	// FeatureGate holds information about enabled features.
	FeatureGate FeatureGate
	// InstrumentationConfig contains the common instrumentation configuration for all controllers.
	InstrumentationConfig InstrumentationConfig
	// ClusterDomain contains the domain of the cluster.
	ClusterDomain string
}

// FeatureGate holds information about enabled features.
type FeatureGate struct {
	// EnableServiceMonitor enables the management of ServiceMonitor objects.
	// See https://prometheus-operator.dev/docs/api-reference/api/#monitoring.coreos.com/v1.ServiceMonitor
	EnableServiceMonitor bool
	// EnablePrometheusRuleDiscovery enables the discovery of PrometheusRule objects to set on Thanos Ruler.
	// See https://prometheus-operator.dev/docs/api-reference/api/#monitoring.coreos.com/v1.PrometheusRule
	EnablePrometheusRuleDiscovery bool
}

// ToGVK returns the GroupVersionKind for all enabled features.
func (fg FeatureGate) ToGVK() []schema.GroupVersionKind {
	var gvk []schema.GroupVersionKind
	if !fg.EnableServiceMonitor {
		gvk = append(gvk, schema.GroupVersionKind{
			Group:   "monitoring.coreos.com",
			Version: "v1",
			Kind:    "ServiceMonitor",
		})
	}
	if !fg.EnablePrometheusRuleDiscovery {
		gvk = append(gvk, schema.GroupVersionKind{
			Group:   "monitoring.coreos.com",
			Version: "v1",
			Kind:    "PrometheusRule",
		})
	}
	return gvk
}

// InstrumentationConfig contains the common instrumentation configuration for all controllers.
type InstrumentationConfig struct {
	Logger        logr.Logger
	EventRecorder record.EventRecorder

	MetricsRegistry prometheus.Registerer

	CommonMetrics *metrics.CommonMetrics
}
