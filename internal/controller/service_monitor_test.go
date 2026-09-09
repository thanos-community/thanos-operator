package controller

import (
	"os"
	"path/filepath"
	"testing"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/compact"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/query"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/queryfrontend"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/receive"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/ruler"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/store"
)

func TestServiceMonitorFileConfig(t *testing.T) {
	for _, tc := range []struct {
		name     string
		enabled  bool
		config   string
		interval monitoringv1.Duration
	}{
		{name: "disabled", config: "service-monitor:\n  interval: invalid"},
		{name: "defaults", enabled: true, config: "service-monitor: {}"},
		{
			name: "configured", enabled: true, interval: "45s",
			config: `service-monitor:
  additionalLabels:
    prometheus: platform
    team: ignored
    app.kubernetes.io/name: ignored
  interval: 45s
`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "feature-gates.yaml")
			assert.NilError(t, os.WriteFile(path, []byte(tc.config), 0600))
			fg := featuregate.Config{
				ServiceMonitor: &featuregate.ServiceMonitorConfig{FeatureConfig: featuregate.FeatureConfig{Enabled: tc.enabled}},
				KubeResourceSync: &featuregate.KubeResourceSyncConfig{
					FeatureConfig: featuregate.FeatureConfig{Enabled: true}, Image: "resource-sync:test",
				},
			}
			fg, err := featuregate.LoadAndApplyConfig(path, fg)
			assert.NilError(t, err)
			owner := &v1alpha1.ThanosQuery{ObjectMeta: metav1.ObjectMeta{
				Name: "test", Namespace: "monitoring", Labels: map[string]string{"owner-label": "keep"},
			}}
			opts := commonToOpts(owner, 1, v1alpha1.CommonFields{Labels: map[string]string{"team": "observability"}}, nil, fg, v1alpha1.Additional{})
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
