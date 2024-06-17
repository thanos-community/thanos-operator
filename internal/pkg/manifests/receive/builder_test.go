package receive

import (
	"testing"

	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/utils/ptr"
)

const (
	someCustomLabelValue string = "xyz"
	someOtherLabelValue  string = "abc"
)

func TestBuildIngesters(t *testing.T) {
	opts := IngesterOptions{
		Options: manifests.Options{
			Name:      "test",
			Namespace: "ns",
			Image:     ptr.To("some-custom-image"),
			Labels: map[string]string{
				"some-custom-label":      someCustomLabelValue,
				"some-other-label":       someOtherLabelValue,
				"app.kubernetes.io/name": "expect-to-be-discarded",
			},
		},
	}

	expectSA := manifests.BuildServiceAccount(opts.Options)
	expectService := NewIngestorService(opts)
	expectStatefulSet := NewIngestorStatefulSet(opts)

	objs := BuildIngesters([]IngesterOptions{opts})
	if len(objs) != 3 {
		t.Fatalf("expected 3 objects, got %d", len(objs))
	}

	if !equality.Semantic.DeepEqual(objs[0], expectSA) {
		t.Errorf("expected first object to be a service account, wanted \n%v\n got \n%v\n", expectSA, objs[0])
	}

	if !equality.Semantic.DeepEqual(objs[1], expectService) {
		t.Errorf("expected second object to be a service, wanted \n%v\n got \n%v\n", expectService, objs[1])
	}

	if !equality.Semantic.DeepEqual(objs[2], expectStatefulSet) {
		t.Errorf("expected third object to be a sts, wanted \n%v\n got \n%v\n", expectStatefulSet, objs[2])
	}

	wantLabels := labelsForIngestor(opts)
	wantLabels["some-custom-label"] = someCustomLabelValue
	wantLabels["some-other-label"] = someOtherLabelValue

	for _, obj := range objs {
		if !equality.Semantic.DeepEqual(obj.GetLabels(), wantLabels) {
			t.Errorf("expected object to have labels %v, got %v", wantLabels, obj.GetLabels())
		}
	}
}

func TestIngesterNameFromParent(t *testing.T) {
	for _, tc := range []struct {
		name   string
		parent string
		child  string
		expect string
	}{
		{
			name:   "test inherit from parent when valid",
			parent: "some-allowed-value",
			child:  "my-resource",
			expect: "some-allowed-value-my-resource",
		},
		{
			name:   "test inherit from parent when invalid",
			parent: "some-disallowed-value-because-the value-is-just-way-too-long-to-be-supported-by-label-constraints-which-are-required-for-matching-ingesters-and-even-though-this-is-unlikely-to-happen-in-practice-we-should-still-handle-it-because-its-possible",
			child:  "my-resource",
			expect: "my-resource",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := IngesterNameFromParent(tc.parent, tc.child); got != tc.expect {
				t.Errorf("expected ingester name to be %s, got %s", tc.expect, got)
			}
		})
	}

}

func TestNewIngestorStatefulSet(t *testing.T) {
	opts := IngesterOptions{
		Options: manifests.Options{
			Name:      "test",
			Namespace: "ns",
			Image:     ptr.To("some-custom-image"),
			Labels: map[string]string{
				"some-custom-label":      someCustomLabelValue,
				"some-other-label":       someOtherLabelValue,
				"app.kubernetes.io/name": "expect-to-be-discarded",
			},
		}.ApplyDefaults(),
	}

	for _, tc := range []struct {
		name string
		opts IngesterOptions
	}{
		{
			name: "test ingester statefulset correctness",
			opts: opts,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.opts.Options = tc.opts.ApplyDefaults()
			ingester := NewIngestorStatefulSet(tc.opts)
			if ingester.GetName() != tc.opts.Name {
				t.Errorf("expected ingester statefulset name to be %s, got %s", tc.opts.Name, ingester.GetName())
			}
			if ingester.GetNamespace() != tc.opts.Namespace {
				t.Errorf("expected ingester statefulset namespace to be %s, got %s", tc.opts.Namespace, ingester.GetNamespace())
			}
			// ensure we inherit the labels from the Options struct and that the strict labels cannot be overridden
			if len(ingester.GetLabels()) != 7 {
				t.Errorf("expected ingester statefulset to have 7 labels, got %d", len(ingester.GetLabels()))
			}
			// ensure custom labels are set
			if ingester.GetLabels()["some-custom-label"] != someCustomLabelValue {
				t.Errorf("expected ingester statefulset to have label 'some-custom-label' with value 'xyz', got %s", ingester.GetLabels()["some-custom-label"])
			}
			if ingester.GetLabels()["some-other-label"] != someOtherLabelValue {
				t.Errorf("expected ingester statefulset to have label 'some-other-label' with value 'abc', got %s", ingester.GetLabels()["some-other-label"])
			}
			// ensure default labels are set
			expect := labelsForIngestor(tc.opts)
			for k, v := range expect {
				if ingester.GetLabels()[k] != v {
					t.Errorf("expected ingester statefulset to have label %s with value %s, got %s", k, v, ingester.GetLabels()[k])
				}
			}

			expectArgs := ingestorArgsFrom(opts)
			var found bool
			for _, c := range ingester.Spec.Template.Spec.Containers {
				if c.Name == IngestComponentName {
					found = true
					if c.Image != tc.opts.GetContainerImage() {
						t.Errorf("expected ingester statefulset to have image %s, got %s", tc.opts.GetContainerImage(), c.Image)
					}
					if len(c.Args) != len(expectArgs) {
						t.Errorf("expected ingester statefulset to have %d args, got %d", len(expectArgs), len(c.Args))
					}
					for i, arg := range c.Args {
						if arg != expectArgs[i] {
							t.Errorf("expected ingester statefulset to have arg %s, got %s", expectArgs[i], arg)
						}
					}
				}
			}
			if !found {
				t.Errorf("expected ingester statefulset to have container named %s", IngestComponentName)
			}
		})
	}
}

func TestNewIngestorService(t *testing.T) {
	opts := IngesterOptions{
		Options: manifests.Options{
			Name:      "test",
			Namespace: "ns",
			Labels: map[string]string{
				"some-custom-label":      someCustomLabelValue,
				"some-other-label":       someOtherLabelValue,
				"app.kubernetes.io/name": "expect-to-be-discarded",
			},
		},
	}

	for _, tc := range []struct {
		name string
		opts IngesterOptions
	}{
		{
			name: "test ingester service correctness",
			opts: opts,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.opts.Options = tc.opts.ApplyDefaults()
			ingester := NewIngestorService(tc.opts)
			if ingester.GetName() != tc.opts.Name {
				t.Errorf("expected ingester service name to be %s, got %s", tc.opts.Name, ingester.GetName())
			}
			if ingester.GetNamespace() != tc.opts.Namespace {
				t.Errorf("expected ingester service namespace to be %s, got %s", tc.opts.Namespace, ingester.GetNamespace())
			}
			// ensure we inherit the labels from the Options struct and that the strict labels cannot be overridden
			if len(ingester.GetLabels()) != 7 {
				t.Errorf("expected ingester service to have 7 labels, got %d", len(ingester.GetLabels()))
			}
			// ensure custom labels are set
			if ingester.GetLabels()["some-custom-label"] != someCustomLabelValue {
				t.Errorf("expected ingester service to have label 'some-custom-label' with value 'xyz', got %s", ingester.GetLabels()["some-custom-label"])
			}
			if ingester.GetLabels()["some-other-label"] != someOtherLabelValue {
				t.Errorf("expected ingester service to have label 'some-other-label' with value 'abc', got %s", ingester.GetLabels()["some-other-label"])
			}
			// ensure default labels are set
			expect := labelsForIngestor(tc.opts)
			for k, v := range expect {
				if ingester.GetLabels()[k] != v {
					t.Errorf("expected ingester service to have label %s with value %s, got %s", k, v, ingester.GetLabels()[k])
				}
			}

			if ingester.Spec.ClusterIP != "None" {
				t.Errorf("expected ingester service to have ClusterIP 'None', got %s", ingester.Spec.ClusterIP)
			}
		})
	}
}
