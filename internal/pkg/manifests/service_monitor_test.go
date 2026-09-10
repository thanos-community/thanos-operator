package manifests_test

import (
	"testing"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/yaml"

	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/compact"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/query"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/queryfrontend"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/receive"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/ruler"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/store"
)

func TestBuildServiceMonitor(t *testing.T) {
	const (
		name = "thanos-stack"
		ns   = "ns"
	)

	randObjMeta := map[string]string{
		"some-random-label": "some-random",
	}
	randSelectorLabels := map[string]string{
		"some-random-selector-label": "some-random",
	}

	for _, tc := range []struct {
		name   string
		golden string
		config featuregate.ServiceMonitorConfig
	}{
		{
			name:   "test service monitor correctness with defaults",
			golden: "servicemonitor-basic.golden.yaml",
			config: featuregate.ServiceMonitorConfig{},
		},
		{
			name:   "test service monitor with custom interval",
			golden: "servicemonitor-custom-interval.golden.yaml",
			config: featuregate.ServiceMonitorConfig{
				Interval: "60s",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sm := manifests.BuildServiceMonitor(name, ns, randObjMeta, randSelectorLabels, tc.config, "http")

			// Test against golden file
			yamlBytes, err := yaml.Marshal(sm)
			if err != nil {
				t.Fatalf("failed to marshal ServiceMonitor to YAML: %v", err)
			}
			golden.Assert(t, string(yamlBytes), tc.golden)
		})
	}
}

func TestComponentServiceMonitors(t *testing.T) {
	for _, tc := range []struct {
		name     string
		enabled  bool
		labels   map[string]string
		interval monitoringv1.Duration
	}{
		{name: "disabled"},
		{name: "defaults", enabled: true},
		{
			name: "configured", enabled: true, interval: "45s",
			labels: map[string]string{
				"prometheus":             "platform",
				"team":                   "ignored",
				"app.kubernetes.io/name": "ignored",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := manifests.Options{
				Owner: "test", Namespace: "monitoring", Replicas: 1,
				Labels: map[string]string{"owner-label": "keep", "team": "observability"},
				Config: featuregate.Config{
					KubeResourceSync: &featuregate.KubeResourceSyncConfig{
						FeatureConfig: featuregate.FeatureConfig{Enabled: true}, Image: "resource-sync:test",
					},
				},
			}
			if tc.enabled {
				opts.ServiceMonitor = &featuregate.ServiceMonitorConfig{
					FeatureConfig:    featuregate.FeatureConfig{Enabled: true},
					AdditionalLabels: tc.labels,
					Interval:         string(tc.interval),
				}
			}
			for _, component := range []struct {
				name     string
				builder  manifests.Buildable
				monitors int
			}{
				{"query", query.Options{Options: opts}, 1},
				{"query frontend", queryfrontend.Options{Options: opts}, 1},
				{"store", store.Options{Options: opts}, 1},
				{"compact", compact.Options{Options: opts}, 1},
				{"ruler", ruler.Options{Options: opts}, 1},
				{"receive ingester", receive.IngesterOptions{Options: opts, HashringName: "default"}, 1},
				{"receive router and resource sync", receive.RouterOptions{Options: opts}, 2},
			} {
				t.Run(component.name, func(t *testing.T) {
					objects := component.builder.Build()
					count := 0
					for _, obj := range objects {
						sm, ok := obj.(*monitoringv1.ServiceMonitor)
						if !ok {
							_, hasExtraLabel := obj.GetLabels()["prometheus"]
							assert.Assert(t, !hasExtraLabel)
							continue
						}
						count++
						assert.Equal(t, sm.Labels["owner-label"], "keep")
						assert.Equal(t, sm.Labels["team"], "observability")
						assert.Equal(t, sm.Labels[manifests.NameLabel], component.builder.GetSelectorLabels()[manifests.NameLabel])
						assert.DeepEqual(t, sm.Spec.Selector.MatchLabels, component.builder.GetSelectorLabels())
						if tc.interval != "" {
							assert.Equal(t, sm.Labels["prometheus"], "platform")
						} else {
							_, hasExtraLabel := sm.Labels["prometheus"]
							assert.Assert(t, !hasExtraLabel)
						}
						assert.Equal(t, len(sm.Spec.Endpoints), 1)
						assert.Equal(t, sm.Spec.Endpoints[0].Interval, tc.interval)
						selector, err := metav1.LabelSelectorAsSelector(&sm.Spec.Selector)
						assert.NilError(t, err)
						matched := false
						for _, target := range objects {
							if svc, ok := target.(*corev1.Service); ok && selector.Matches(labels.Set(svc.Labels)) {
								for _, port := range svc.Spec.Ports {
									matched = matched || port.Name == sm.Spec.Endpoints[0].Port
								}
							}
						}
						assert.Assert(t, matched, "monitor must select a Service exposing its metrics port")
					}
					want := component.monitors
					if !tc.enabled {
						want = 0
					}
					assert.Equal(t, count, want)
				})
			}
			_, hasExtraLabel := opts.Labels["prometheus"]
			assert.Assert(t, !hasExtraLabel)
		})
	}
}
