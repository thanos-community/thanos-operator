package manifests

import (
	"testing"

	"gotest.tools/v3/golden"
	"sigs.k8s.io/yaml"
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
		name   string
		golden string
		opts   Options
	}{
		{
			name:   "test service account correctness",
			golden: "serviceaccount-basic.golden.yaml",
			opts:   opts,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sa := BuildServiceAccount(name, tc.opts.Namespace, tc.opts.Labels, tc.opts.Annotations)

			// Test against golden file
			yamlBytes, err := yaml.Marshal(sa)
			if err != nil {
				t.Fatalf("failed to marshal service account to YAML: %v", err)
			}
			golden.Assert(t, string(yamlBytes), tc.golden)
		})
	}
}
