package manifests

import (
	"testing"
)

func TestBuildServiceAccount(t *testing.T) {
	const name = "thanos-stack"
	opts := Options{
		Namespace: "ns",
		Labels: map[string]string{
			"app.kubernetes.io/name":     "thanos",
			"app.kubernetes.io/instance": "thanos-stack",
		},
		Annotations: map[string]string{
			"test": "annotation",
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
			sa := BuildServiceAccount(name, tc.opts.Namespace, tc.opts.Labels, tc.opts.Annotations)
			if sa.GetName() != name {
				t.Errorf("expected service account name to be %s, got %s", name, sa.GetName())
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
			if len(sa.GetAnnotations()) != 1 {
				t.Errorf("expected service account to have 1 annotation, got %d", len(sa.GetAnnotations()))
			}
			if sa.GetAnnotations()["test"] != "annotation" {
				t.Errorf("expected service account annotation test to be annotation, got %s", sa.GetAnnotations()["test"])
			}
		})
	}
}
