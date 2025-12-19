package manifests

import (
	"testing"

	"gotest.tools/v3/golden"
	"sigs.k8s.io/yaml"
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
		opts   ServiceMonitorOptions
	}{
		{
			name:   "test service monitor correctness with defaults",
			golden: "servicemonitor-basic.golden.yaml",
			opts:   ServiceMonitorOptions{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sm := BuildServiceMonitor(name, ns, randObjMeta, randSelectorLabels, tc.opts)

			// Test against golden file
			yamlBytes, err := yaml.Marshal(sm)
			if err != nil {
				t.Fatalf("failed to marshal ServiceMonitor to YAML: %v", err)
			}
			golden.Assert(t, string(yamlBytes), tc.golden)
		})
	}
}
