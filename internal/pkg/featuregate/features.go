package featuregate

import "slices"

// Feature flag names for use with --enable-feature flag.
// These follow Prometheus convention of kebab-case feature names.
const (
	// ServiceMonitor enables management of ServiceMonitor objects.
	// See https://prometheus-operator.dev/docs/api-reference/api/#monitoring.coreos.com/v1.ServiceMonitor
	ServiceMonitor = "service-monitor"

	// PrometheusRule enables discovery of PrometheusRule objects to set on Thanos Ruler.
	// See https://prometheus-operator.dev/docs/api-reference/api/#monitoring.coreos.com/v1.PrometheusRule
	PrometheusRule = "prometheus-rule"

	// OtelSidecar enables OpenTelemetry collector sidecar injection for Thanos components.
	// This allows automatic injection of OpenTelemetry collectors into Thanos pods for tracing.
	OtelSidecar = "otel-sidecar"

	// KubeResourceSync enables the kube-resource-sync sidecar for immediate ConfigMap/Secret synchronization.
	// See https://github.com/philipgough/kube-resource-sync
	KubeResourceSync = "kube-resource-sync"

	// VolumeResize enables the volume resize controller for automatic PVC resizing.
	VolumeResize = "volume-resize"

	// ServerTLS enables server TLS between Thanos components.
	ServerTLS = "server-tls"
)

const defaultKubeResourceSyncImage = "quay.io/philipgough/kube-resource-sync:0.1.0"

// AllFeatures returns a slice of all available feature flag names.
// This is useful for validation and help text generation.
func AllFeatures() []string {
	return []string{
		ServiceMonitor,
		PrometheusRule,
		KubeResourceSync,
		OtelSidecar,
		VolumeResize,
		ServerTLS,
	}
}

// IsValidFeature checks if a given feature name is valid.
func IsValidFeature(feature string) bool {
	return slices.Contains(AllFeatures(), feature)
}

// FeatureConfig is the base configuration shared by all features.
// Features that need extra settings embed this and add their own fields.
type FeatureConfig struct {
	// Enabled indicates whether the feature is turned on.
	Enabled bool
}

// KubeResourceSyncConfig configures the kube-resource-sync feature.
type KubeResourceSyncConfig struct {
	FeatureConfig
	// Image is the container image used for the kube-resource-sync sidecar.
	Image string
}

// ServiceMonitorConfig configures generated ServiceMonitors.
type ServiceMonitorConfig struct {
	FeatureConfig
	// AdditionalLabels are added to ServiceMonitor metadata.
	AdditionalLabels map[string]string
	// Interval overrides the Prometheus scrape interval when set.
	Interval string
}

// Config holds information about globally enabled features.
// This represents the actual feature state used by controllers and manifest builders.
// A nil pointer means the feature is not configured, which is treated as disabled.
type Config struct {
	// ServiceMonitor configures management of ServiceMonitor objects.
	ServiceMonitor *ServiceMonitorConfig
	// PrometheusRule configures discovery of PrometheusRule objects.
	PrometheusRule *FeatureConfig
	// OtelSidecar configures OpenTelemetry collector sidecar injection.
	OtelSidecar *FeatureConfig
	// KubeResourceSync configures the kube-resource-sync sidecar container.
	KubeResourceSync *KubeResourceSyncConfig
	// VolumeResize configures the volume resize controller.
	VolumeResize *FeatureConfig
	// ServerTLS configures certificates and trust for Thanos components.
	ServerTLS *ServerTLSConfig
}

// Enabled returns a pointer to an enabled FeatureConfig.
// It is a convenience helper for constructing a Config.
func Enabled() *FeatureConfig {
	return &FeatureConfig{Enabled: true}
}

// ServiceMonitorEnabled returns true if ServiceMonitor management is enabled.
func (c Config) ServiceMonitorEnabled() bool {
	return c.ServiceMonitor != nil && c.ServiceMonitor.Enabled
}

// PrometheusRuleEnabled returns true if PrometheusRule discovery is enabled.
func (c Config) PrometheusRuleEnabled() bool {
	return c.PrometheusRule != nil && c.PrometheusRule.Enabled
}

// OtelSidecarEnabled returns true if OpenTelemetry sidecar injection is enabled.
func (c Config) OtelSidecarEnabled() bool {
	return c.OtelSidecar != nil && c.OtelSidecar.Enabled
}

// KubeResourceSyncEnabled returns true if KubeResourceSync sidecar is enabled.
func (c Config) KubeResourceSyncEnabled() bool {
	return c.KubeResourceSync != nil && c.KubeResourceSync.Enabled
}

// VolumeResizeEnabled returns true if volume resize controller is enabled.
func (c Config) VolumeResizeEnabled() bool {
	return c.VolumeResize != nil && c.VolumeResize.Enabled
}

func (c Config) ServerTLSEnabled() bool {
	return c.ServerTLS != nil && c.ServerTLS.Enabled
}

// GetKubeResourceSyncImage returns the image used for the kube-resource-sync sidecar.
func (c Config) GetKubeResourceSyncImage() string {
	if c.KubeResourceSync == nil {
		return ""
	}
	return c.KubeResourceSync.Image
}

// ToFeatureGate converts a Flag to a Config struct for use by controllers.
func (f *Flag) ToFeatureGate() Config {
	var c Config
	if f.EnablesServiceMonitor() {
		c.ServiceMonitor = &ServiceMonitorConfig{FeatureConfig: FeatureConfig{Enabled: true}}
	}
	if f.EnablesPrometheusRule() {
		c.PrometheusRule = Enabled()
	}
	if f.EnablesOtelSidecar() {
		c.OtelSidecar = Enabled()
	}
	if f.EnablesKubeResourceSync() {
		c.KubeResourceSync = &KubeResourceSyncConfig{FeatureConfig: FeatureConfig{Enabled: true}}
	}
	if f.EnablesVolumeResize() {
		c.VolumeResize = Enabled()
	}
	if f.Contains(ServerTLS) {
		c.ServerTLS = &ServerTLSConfig{FeatureConfig: FeatureConfig{Enabled: true}, Provider: CertManagerProvider}
	}
	return c
}
