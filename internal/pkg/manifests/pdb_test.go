package manifests

import (
	"testing"

	"k8s.io/utils/ptr"
)

func TestNewPodDisruptionBudget(t *testing.T) {
	type args struct {
		name             string
		namespace        string
		selectorLabels   map[string]string
		objectMetaLabels map[string]string
		maxUnavailable   int
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Test NewPodDisruptionBudget",
			args: args{
				name:             "test-name",
				namespace:        "test-namespace",
				selectorLabels:   map[string]string{"test": "label"},
				objectMetaLabels: map[string]string{"test": "label"},
				maxUnavailable:   1,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mu := ptr.To(tt.args.maxUnavailable)
			pdb := NewPodDisruptionBudget(tt.args.name, tt.args.namespace, tt.args.selectorLabels, tt.args.objectMetaLabels, mu)
			if pdb.Name != tt.args.name {
				t.Errorf("pdb.Name = %v, want %v", pdb.Name, tt.args.name)
			}
			if pdb.Namespace != tt.args.namespace {
				t.Errorf("pdb.Namespace = %v, want %v", pdb.Namespace, tt.args.namespace)
			}
			if pdb.Spec.MinAvailable.IntVal != int32(tt.args.maxUnavailable) {
				t.Errorf("pdb.Spec.MinAvailable.IntVal = %v, want %v", pdb.Spec.MinAvailable.IntVal, tt.args.maxUnavailable)
			}
		})
	}
}
