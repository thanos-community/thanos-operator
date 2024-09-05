package manifests

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func TestMergeLabels(t *testing.T) {
	for _, tc := range []struct {
		name      string
		labels    map[string]string
		mergeWith map[string]string
		expect    map[string]string
	}{
		{
			name: "merge labels with priority",
			labels: map[string]string{
				"a":   "b",
				"foo": "bar",
			},
			mergeWith: map[string]string{
				"bar": "baz",
				"foo": "baz",
			},
			expect: map[string]string{
				"a":   "b",
				"foo": "baz",
				"bar": "baz",
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := MergeLabels(tc.labels, tc.mergeWith)
			if len(result) != len(tc.expect) {
				t.Fatalf("expected %d labels, got %d", len(tc.expect), len(result))
			}
			for k, v := range tc.expect {
				if result[k] != v {
					t.Errorf("expected label %s to be %s, got %s", k, v, result[k])
				}
			}
		})
	}
}

func TestBuildLabelSelectorFrom(t *testing.T) {
	for _, tc := range []struct {
		name          string
		labelSelector *metav1.LabelSelector
		required      map[string]string
		expect        map[string]string
	}{
		{
			name:          "build label selector from nil label selector",
			labelSelector: nil,
			required: map[string]string{
				"a": "b",
			},
			expect: map[string]string{
				"a": "b",
			}},
		{
			name: "build label selector from existing label selector",
			labelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"foo": "bar",
				},
			},
			required: map[string]string{
				"a": "b",
			},
			expect: map[string]string{
				"foo": "bar",
				"a":   "b",
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := BuildLabelSelectorFrom(tc.labelSelector, tc.required)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result.String()) == 0 {
				t.Fatalf("expected label selector to be non-empty")
			}

			if !result.Matches(labels.Set(tc.expect)) {
				t.Errorf("expected label selector to match %v", tc.expect)
			}
		})
	}
}
