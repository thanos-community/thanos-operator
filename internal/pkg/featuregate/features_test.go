package featuregate

import (
	"reflect"
	"testing"
)

func TestAllFeatures(t *testing.T) {
	expected := []string{
		"service-monitor",
		"prometheus-rule",
		"otel-sidecar",
		"kube-resource-sync",
	}

	got := AllFeatures()
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("AllFeatures() = %v, want %v", got, expected)
	}
}

func TestIsValidFeature(t *testing.T) {
	tests := []struct {
		name    string
		feature string
		want    bool
	}{
		{
			name:    "valid service-monitor",
			feature: ServiceMonitor,
			want:    true,
		},
		{
			name:    "valid prometheus-rule",
			feature: PrometheusRule,
			want:    true,
		},
		{
			name:    "valid otel-sidecar",
			feature: OtelSidecar,
			want:    true,
		},
		{
			name:    "invalid feature",
			feature: "invalid-feature",
			want:    false,
		},
		{
			name:    "empty feature",
			feature: "",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidFeature(tt.feature); got != tt.want {
				t.Errorf("IsValidFeature(%q) = %v, want %v", tt.feature, got, tt.want)
			}
		})
	}
}

func TestConfig_OtelSidecarEnabled(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   bool
	}{
		{
			name:   "otel sidecar enabled",
			config: Config{EnableOtelSidecar: true},
			want:   true,
		},
		{
			name:   "otel sidecar disabled",
			config: Config{EnableOtelSidecar: false},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.OtelSidecarEnabled(); got != tt.want {
				t.Errorf("Config.OtelSidecarEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFlag_ToFeatureGate(t *testing.T) {
	tests := []struct {
		name     string
		features []string
		want     Config
	}{
		{
			name:     "no features",
			features: []string{},
			want: Config{
				EnableServiceMonitor:          false,
				EnablePrometheusRuleDiscovery: false,
				EnableOtelSidecar:             false,
			},
		},
		{
			name:     "all features enabled",
			features: []string{ServiceMonitor, PrometheusRule, OtelSidecar},
			want: Config{
				EnableServiceMonitor:          true,
				EnablePrometheusRuleDiscovery: true,
				EnableOtelSidecar:             true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Flag{}
			for _, feature := range tt.features {
				_ = f.Set(feature)
			}

			if got := f.ToFeatureGate(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Flag.ToFeatureGate() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
