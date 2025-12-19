package manifests

import (
	"testing"

	"gotest.tools/v3/golden"
	"sigs.k8s.io/yaml"
)

func TestNewPodDisruptionBudget(t *testing.T) {
	type args struct {
		name             string
		namespace        string
		selectorLabels   map[string]string
		objectMetaLabels map[string]string
		annotations      map[string]string
		conf             PodDisruptionBudgetOptions
	}
	tests := []struct {
		name   string
		golden string
		args   args
	}{
		{
			name:   "Test NewPodDisruptionBudget",
			golden: "pdb-basic.golden.yaml",
			args: args{
				name:             "test-name",
				namespace:        "test-namespace",
				selectorLabels:   map[string]string{"test": "label"},
				objectMetaLabels: map[string]string{"test": "label"},
				annotations:      map[string]string{"test": "annotation"},
				conf:             PodDisruptionBudgetOptions{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdb := NewPodDisruptionBudget(tt.args.name, tt.args.namespace, tt.args.selectorLabels, tt.args.objectMetaLabels, tt.args.annotations, tt.args.conf)

			// Test against golden file
			yamlBytes, err := yaml.Marshal(pdb)
			if err != nil {
				t.Fatalf("failed to marshal PodDisruptionBudget to YAML: %v", err)
			}
			golden.Assert(t, string(yamlBytes), tt.golden)
		})
	}
}
