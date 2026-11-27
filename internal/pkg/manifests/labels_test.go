package manifests

import (
	"maps"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func TestMergeMaps(t *testing.T) {
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
			result := MergeMaps(tc.labels, tc.mergeWith)
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
		expect        labels.Set
		reject        []labels.Set
		wantErr       bool
	}{
		{
			name:          "build label selector from nil label selector",
			labelSelector: nil,
			required: map[string]string{
				"a": "b",
			},
			expect: map[string]string{
				"a": "b",
			},
			reject: []labels.Set{{}, {"a": "other"}},
		},
		{
			name:   "nil selector without required labels matches everything",
			expect: labels.Set{},
		},
		{
			name:          "empty selector with required labels",
			labelSelector: &metav1.LabelSelector{},
			required:      map[string]string{"a": "b"},
			expect:        labels.Set{"a": "b"},
			reject:        []labels.Set{{}, {"a": "other"}},
		},
		{
			name:          "empty selector without required labels matches everything",
			labelSelector: &metav1.LabelSelector{},
			expect:        labels.Set{},
		},
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
			},
			reject: []labels.Set{{"foo": "bar"}, {"a": "b"}},
		},
		{
			name: "expression-only selector with required labels",
			labelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: "component", Operator: metav1.LabelSelectorOpIn, Values: []string{"store", "receive"}},
				},
			},
			required: map[string]string{"a": "b"},
			expect:   labels.Set{"a": "b", "component": "store"},
			reject:   []labels.Set{{"component": "store"}, {"a": "b"}, {"a": "b", "component": "query"}},
		},
		{
			name: "expression-only selector without required labels",
			labelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: "component", Operator: metav1.LabelSelectorOpExists},
				},
			},
			expect: labels.Set{"component": "store"},
			reject: []labels.Set{{}},
		},
		{
			name: "labels and expressions are combined with required labels",
			labelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"team": "metrics"},
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: "disabled", Operator: metav1.LabelSelectorOpDoesNotExist},
				},
			},
			required: map[string]string{"a": "b"},
			expect:   labels.Set{"a": "b", "team": "metrics"},
			reject: []labels.Set{
				{"team": "metrics"},
				{"a": "b"},
				{"a": "b", "team": "metrics", "disabled": "true"},
			},
		},
		{
			name: "required labels retain their existing precedence",
			labelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"a": "other"},
			},
			required: map[string]string{"a": "b"},
			expect:   labels.Set{"a": "b"},
			reject:   []labels.Set{{"a": "other"}},
		},
		{
			name: "invalid expression returns an error",
			labelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: "component", Operator: "Invalid"},
				},
			},
			required: map[string]string{"a": "b"},
			wantErr:  true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			originalSelector := tc.labelSelector.DeepCopy()
			originalRequired := maps.Clone(tc.required)
			result, err := BuildLabelSelectorFrom(tc.labelSelector, tc.required)
			if !reflect.DeepEqual(tc.labelSelector, originalSelector) {
				t.Error("input selector was modified")
			}
			if !reflect.DeepEqual(tc.required, originalRequired) {
				t.Error("required labels were modified")
			}
			if (err != nil) != tc.wantErr {
				t.Fatalf("got error %v, want error: %t", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if !result.Matches(tc.expect) {
				t.Errorf("expected label selector to match %v", tc.expect)
			}
			for _, rejected := range tc.reject {
				if result.Matches(rejected) {
					t.Errorf("expected label selector not to match %v", rejected)
				}
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
			SanitizeStoreAPIEndpointLabels(test.input)
			if !reflect.DeepEqual(test.input, test.expected) {
				t.Errorf("got %v, want %v", test.input, test.expected)
			}
		})
	}
}
