package manifests

import "testing"

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
