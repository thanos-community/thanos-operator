package manifests

import (
	"testing"
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
		name string
		opts ServiceMonitorOptions
	}{
		{
			name: "test service monitor correctness with defaults",
			opts: ServiceMonitorOptions{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sm := BuildServiceMonitor(name, ns, randObjMeta, randSelectorLabels, tc.opts)
			if sm.GetName() != name {
				t.Errorf("expected service monitor name to be %s, got %s", name, sm.GetName())
			}
			if sm.GetNamespace() != ns {
				t.Errorf("expected service monitor namespace to be %s, got %s", ns, sm.GetNamespace())
			}
			if len(sm.Spec.Selector.MatchLabels) != 1 {
				t.Errorf("expected service monitor to have 1 match labels, got %d", len(sm.Spec.Selector.MatchLabels))
			}
			if len(sm.Spec.NamespaceSelector.MatchNames) != 1 {
				t.Errorf("expected service monitor to have 1 match name, got %d", len(sm.Spec.NamespaceSelector.MatchNames))
			}
			if sm.Spec.NamespaceSelector.MatchNames[0] != ns {
				t.Errorf("expected service monitor match name to be %s, got %s", ns, sm.Spec.NamespaceSelector.MatchNames[0])
			}
			if len(sm.Spec.Endpoints) != 1 {
				t.Errorf("expected service monitor to have 1 endpoint, got %d", len(sm.Spec.Endpoints))
			}
			if sm.Spec.Endpoints[0].Port != "http" {
				t.Errorf("expected service monitor endpoint port to be http, got %s", sm.Spec.Endpoints[0].Port)
			}
			if sm.Spec.Endpoints[0].Path != "/metrics" {
				t.Errorf("expected service monitor endpoint path to be /metrics, got %s", sm.Spec.Endpoints[0].Path)
			}
		})
	}
}
