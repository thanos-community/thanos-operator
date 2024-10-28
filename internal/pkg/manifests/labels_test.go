package manifests

import (
	"reflect"
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
			t.Parallel()
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
			t.Parallel()
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

func TestSanitizeStoreAPIEndpointLabels(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		expected map[string]string
	}{
		{
			name: "Keep GroupStrictLabel and remove others",
			input: map[string]string{
				"operator.thanos.io/endpoint":              "value1",
				"operator.thanos.io/endpoint-strict":       "value2",
				"operator.thanos.io/endpoint-group":        "value3",
				"operator.thanos.io/endpoint-group-strict": "value4",
			},
			expected: map[string]string{
				"operator.thanos.io/endpoint-group-strict": "value4",
			},
		},
		{
			name: "Keep GroupLabel when GroupStrictLabel is absent",
			input: map[string]string{
				"operator.thanos.io/endpoint":       "value1",
				"operator.thanos.io/endpoint-group": "value3",
			},
			expected: map[string]string{
				"operator.thanos.io/endpoint-group": "value3",
			},
		},
		{
			name: "Keep StrictLabel when no group labels present",
			input: map[string]string{
				"operator.thanos.io/endpoint":        "value1",
				"operator.thanos.io/endpoint-strict": "value2",
			},
			expected: map[string]string{
				"operator.thanos.io/endpoint-strict": "value2",
			},
		},
		{
			name: "Keep RegularLabel when no other labels present",
			input: map[string]string{
				"operator.thanos.io/endpoint": "value1",
			},
			expected: map[string]string{
				"operator.thanos.io/endpoint": "value1",
			},
		},
		{
			name: "Do not remove unrelated labels",
			input: map[string]string{
				"operator.thanos.io/endpoint":              "value1",
				"operator.thanos.io/endpoint-strict":       "value2",
				"operator.thanos.io/endpoint-group":        "value3",
				"some.other/label":                         "otherValue",
				"operator.thanos.io/endpoint-group-strict": "value4",
			},
			expected: map[string]string{
				"operator.thanos.io/endpoint-group-strict": "value4",
				"some.other/label":                         "otherValue",
			},
		},
		{
			name: "Do not remove labels when no endpoint labels are set",
			input: map[string]string{
				"some.other/label": "otherValue",
			},
			expected: map[string]string{
				"some.other/label": "otherValue",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			SanitizeStoreAPIEndpointLabels(test.input)
			if !reflect.DeepEqual(test.input, test.expected) {
				t.Errorf("got %v, want %v", test.input, test.expected)
			}
		})
	}
}
