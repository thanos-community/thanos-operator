package manifests

import (
	"testing"
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
				annotations:      map[string]string{"test": "annotation"},
				conf:             PodDisruptionBudgetOptions{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdb := NewPodDisruptionBudget(tt.args.name, tt.args.namespace, tt.args.selectorLabels, tt.args.objectMetaLabels, tt.args.annotations, tt.args.conf)
			if pdb.Name != tt.args.name {
				t.Errorf("pdb.Name = %v, want %v", pdb.Name, tt.args.name)
			}
			if pdb.Namespace != tt.args.namespace {
				t.Errorf("pdb.Namespace = %v, want %v", pdb.Namespace, tt.args.namespace)
			}
			if pdb.Spec.MaxUnavailable.IntVal != int32(1) {
				t.Errorf("pdb.Spec.MinAvailable.IntVal = %v, want %v", pdb.Spec.MaxUnavailable.IntVal, 1)
			}
		})
	}
}
