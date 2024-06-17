package manifests

import (
	"testing"
)

func TestBuildServiceAccount(t *testing.T) {
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
			sa := BuildServiceAccount(tc.opts)
			if sa.GetName() != tc.opts.Name {
				t.Errorf("expected service account name to be %s, got %s", tc.opts.Name, sa.GetName())
			}
			if sa.GetNamespace() != tc.opts.Namespace {
				t.Errorf("expected service account namespace to be %s, got %s", tc.opts.Namespace, sa.GetNamespace())
			}
			if len(sa.GetLabels()) != 2 {
				t.Errorf("expected service account to have 2 labels, got %d", len(sa.GetLabels()))
			}
			if sa.GetLabels()["app.kubernetes.io/name"] != "thanos" {
				t.Errorf("expected service account label app.kubernetes.io/name to be thanos, got %s", sa.GetLabels()["app.kubernetes.io/name"])
			}
			if sa.GetLabels()["app.kubernetes.io/instance"] != "thanos-stack" {
				t.Errorf("expected service account label app.kubernetes.io/instance to be thanos-stack, got %s", sa.GetLabels()["app.kubernetes.io/instance"])
			}
		})
	}
}
