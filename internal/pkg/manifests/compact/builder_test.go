package compact

import (
	"testing"

	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	someCustomLabelValue string = "xyz"
	someOtherLabelValue  string = "abc"
)

func TestNewService(t *testing.T) {
	opts := Options{
		Options: manifests.Options{
			Namespace: "any",
			Name:      "standalone",
		},
	}
	obj := NewService(opts)

	if obj.GetName() != "standalone" {
		t.Errorf("expected service name to be 'standalone', got %s", obj.GetName())
	}
	if obj.GetNamespace() != "any" {
		t.Errorf("expected service namespace to be 'any', got %s", obj.GetNamespace())
	}

	k8Svc := obj.(*corev1.Service)
	if k8Svc.Spec.Ports[0].Name != "http" {
		t.Errorf("expected service port name to be 'http', got %s", k8Svc.Spec.Ports[0].Name)
	}
	if k8Svc.Spec.Ports[0].Port != HTTPPort {
		t.Errorf("expected service port to be %d, got %d", HTTPPort, k8Svc.Spec.Ports[0].Port)
	}

	opts = Options{
		Options: manifests.Options{
			Namespace: "any",
			Name:      "standalone",
		},
		ShardName: "shard",
	}
	obj = NewService(opts)

	if obj.GetName() != "shard" {
		t.Errorf("expected service name to be 'shard', got %s", obj.GetName())
	}
}

func TestNewStatefulSet(t *testing.T) {
	for _, tc := range []struct {
		name       string
		opts       Options
		expectName string
	}{
		{
			name:       "test compact sts correctness with no shard name",
			expectName: "test",
			opts: Options{
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
				ShardName: "test",
			},
		},
		{
			name:       "test compact sts correctness with no shard name",
			expectName: "some-shard",
			opts: Options{
				ShardName: "some-shard",
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
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			compact := NewStatefulSet(tc.opts)
			if compact.GetName() != tc.expectName {
				t.Errorf("expected compact statefulset name to be %s, got %s", tc.expectName, compact.GetName())
			}
			if compact.GetNamespace() != tc.opts.Namespace {
				t.Errorf("expected compact statefulset namespace to be %s, got %s", tc.opts.Namespace, compact.GetNamespace())
			}
			// ensure we inherit the labels from the Options struct and that the strict labels cannot be overridden
			if len(compact.GetLabels()) != 8 {
				t.Errorf("expected compact statefulset to have 8 labels, got %d", len(compact.GetLabels()))
			}
			// ensure custom labels are set
			if compact.GetLabels()["some-custom-label"] != someCustomLabelValue {
				t.Errorf("expected compact statefulset to have label 'some-custom-label' with value 'xyz', got %s", compact.GetLabels()["some-custom-label"])
			}
			if compact.GetLabels()["some-other-label"] != someOtherLabelValue {
				t.Errorf("expected compact statefulset to have label 'some-other-label' with value 'abc', got %s", compact.GetLabels()["some-other-label"])
			}
			// ensure default labels are set
			expect := labelsForCompactorShard(tc.opts)
			for k, v := range expect {
				if compact.GetLabels()[k] != v {
					t.Errorf("expected compact statefulset to have label %s with value %s, got %s", k, v, compact.GetLabels()[k])
				}
			}

			compactSts := compact.(*appsv1.StatefulSet)
			if tc.name == "test additional container" && len(compactSts.Spec.Template.Spec.Containers) != 2 {
				t.Errorf("expected compact statefulset to have 2 containers, got %d", len(compactSts.Spec.Template.Spec.Containers))
			}

			expectArgs := compactorArgsFrom(tc.opts)
			var found bool
			for _, c := range compactSts.Spec.Template.Spec.Containers {
				if c.Name == Name {
					found = true
					if c.Image != tc.opts.GetContainerImage() {
						t.Errorf("expected compact statefulset to have image %s, got %s", tc.opts.GetContainerImage(), c.Image)
					}
					if len(c.Args) != len(expectArgs) {
						t.Errorf("expected compact statefulset to have %d args, got %d", len(expectArgs), len(c.Args))
					}
					for i, arg := range c.Args {
						if arg != expectArgs[i] {
							t.Errorf("expected compact statefulset to have arg %s, got %s", expectArgs[i], arg)
						}
					}

					if tc.name == "test additional volumemount" {
						if len(c.VolumeMounts) != 2 {
							t.Errorf("expected compact statefulset to have 2 volumemount, got %d", len(c.VolumeMounts))
						}
						if c.VolumeMounts[0].Name != "data" {
							t.Errorf("expected compact statefulset to have volumemount named data, got %s", c.VolumeMounts[0].Name)
						}
						if c.VolumeMounts[0].MountPath != "var/thanos/store" {
							t.Errorf("expected compact statefulset to have volumemount mounted at var/thanos/compact, got %s", c.VolumeMounts[0].MountPath)
						}
					}
				}
			}
			if !found {
				t.Errorf("expected compact statefulset to have container named %s", Name)
			}
		})
	}
}

func TestBuild(t *testing.T) {
	opts := Options{
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
		ShardName: "test",
	}

	expectService := NewService(opts)
	expectStatefulSet := NewStatefulSet(opts)

	objs := Build(opts)
	if len(objs) != 3 {
		t.Fatalf("expected 3 objects, got %d", len(objs))
	}

	if !equality.Semantic.DeepEqual(objs[1], expectStatefulSet) {
		t.Errorf("expected second object to be a statefulset, wanted \n%v\n got \n%v\n", expectStatefulSet, objs[1])
	}

	if !equality.Semantic.DeepEqual(objs[2], expectService) {
		t.Errorf("expected third object to be a service, wanted \n%v\n got \n%v\n", expectService, objs[2])
	}

	wantLabels := labelsForCompactorShard(opts)
	wantLabels["some-custom-label"] = someCustomLabelValue
	wantLabels["some-other-label"] = someOtherLabelValue

	for _, obj := range []client.Object{objs[1], objs[2]} {
		if !equality.Semantic.DeepEqual(obj.GetLabels(), wantLabels) {
			t.Errorf("expected object to have labels %v, got %v", wantLabels, obj.GetLabels())
		}
	}
}
