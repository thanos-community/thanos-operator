package manifests

import "testing"

func TestBuildServiceMonitor(t *testing.T) {
	opts := Options{
		Name:      "thanos-stack",
		Namespace: "ns",
		Labels: map[string]string{
			"app.kubernetes.io/name":     "thanos",
			"app.kubernetes.io/instance": "thanos-stack",
		},
	}
	for _, tc := range []struct {
		name string
		opts Options
	}{
		{
			name: "test service account correctness",
			opts: opts,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sm := BuildServiceMonitor(tc.opts, "9090")
			if sm.GetName() != tc.opts.Name {
				t.Errorf("expected service monitor name to be %s, got %s", tc.opts.Name, sm.GetName())
			}
			if sm.GetNamespace() != tc.opts.Namespace {
				t.Errorf("expected service monitor namespace to be %s, got %s", tc.opts.Namespace, sm.GetNamespace())
			}
			if len(sm.Spec.Selector.MatchLabels) != 2 {
				t.Errorf("expected service monitor to have 2 match labels, got %d", len(sm.Spec.Selector.MatchLabels))
			}
			if sm.Spec.Selector.MatchLabels["app.kubernetes.io/name"] != "thanos" {
				t.Errorf("expected service monitor match label app.kubernetes.io/name to be thanos, got %s", sm.Spec.Selector.MatchLabels["app.kubernetes.io/name"])
			}
			if sm.Spec.Selector.MatchLabels["app.kubernetes.io/instance"] != "thanos-stack" {
				t.Errorf("expected service monitor match label app.kubernetes.io/instance to be thanos-stack, got %s", sm.Spec.Selector.MatchLabels["app.kubernetes.io/instance"])
			}
			if len(sm.Spec.NamespaceSelector.MatchNames) != 1 {
				t.Errorf("expected service monitor to have 1 match name, got %d", len(sm.Spec.NamespaceSelector.MatchNames))
			}
			if sm.Spec.NamespaceSelector.MatchNames[0] != tc.opts.Namespace {
				t.Errorf("expected service monitor match name to be %s, got %s", tc.opts.Namespace, sm.Spec.NamespaceSelector.MatchNames[0])
			}
			if len(sm.Spec.Endpoints) != 1 {
				t.Errorf("expected service monitor to have 1 endpoint, got %d", len(sm.Spec.Endpoints))
			}
			if sm.Spec.Endpoints[0].Port != "9090" {
				t.Errorf("expected service monitor endpoint port to be 9090, got %s", sm.Spec.Endpoints[0].Port)
			}
			if sm.Spec.Endpoints[0].Path != "/metrics" {
				t.Errorf("expected service monitor endpoint path to be /metrics, got %s", sm.Spec.Endpoints[0].Path)
			}
		})
	}
}
