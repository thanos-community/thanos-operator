package store

import (
	"testing"

	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/utils/ptr"
)

const (
	someCustomLabelValue string = "xyz"
	someOtherLabelValue  string = "abc"
)

func TestBuildStore(t *testing.T) {
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
		}.ApplyDefaults(),
		Shards: 1,
	}

	expectServices := NewStoreServices(opts)
	expectStatefulsets := NewStoreStatefulSets(opts)

	objs := BuildStores(opts)
	if len(objs) != 4 {
		t.Fatalf("expected 4 objects, got %d", len(objs))
	}

	if objs[0].GetObjectKind().GroupVersionKind().String() != "ServiceAccount" && objs[0].GetName() != "test" {
		t.Errorf("expected first object to be a service account, got %v", objs[0])
	}

	if !equality.Semantic.DeepEqual(objs[0].GetLabels(), objs[1].GetLabels()) {
		t.Errorf("expected service account and other resource to have the same labels, got %v and %v", objs[0].GetLabels(), objs[1].GetLabels())
	}

	for _, sts := range expectStatefulsets {
		if !equality.Semantic.DeepEqual(objs[2], sts) {
			t.Errorf("expected second object to be a statefulset, wanted \n%v\n got \n%v\n", sts, objs[2])
		}
	}

	for _, svc := range expectServices {
		if !equality.Semantic.DeepEqual(objs[1], svc) {
			t.Errorf("expected third object to be a service, wanted \n%v\n got \n%v\n", svc, objs[1])
		}
	}

	wantLabels := labelsForStoreShard(opts)
	wantLabels["some-custom-label"] = someCustomLabelValue
	wantLabels["some-other-label"] = someOtherLabelValue

	for _, obj := range objs {
		if !equality.Semantic.DeepEqual(obj.GetLabels(), wantLabels) {
			t.Errorf("expected object to have labels %v, got %v", wantLabels, obj.GetLabels())
		}
	}
}

func TestNewStoreStatefulSet(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts Options
	}{
		{
			name: "test store stateful correctness",
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
				}.ApplyDefaults(),
				Shards: 1,
			},
		},
		{
			name: "test additional volumemount",
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
				}.ApplyDefaults(),
				Shards: 1,
				Additional: manifests.Additional{
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "test-sd",
							MountPath: "/test-sd-file",
						},
					},
				},
			},
		},
		{
			name: "test additional container",
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
				}.ApplyDefaults(),
				Shards: 1,
				Additional: manifests.Additional{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "test-image:latest",
							Args:  []string{"--test-arg"},
							Env: []corev1.EnvVar{{
								Name:  "TEST_ENV",
								Value: "test",
							}},
						},
					},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := NewStoreStatefulSets(tc.opts)
			if store[0].GetName() != (tc.opts.Name + "-shard-0") {
				t.Errorf("expected store statefulset name to be %s, got %s", tc.opts.Name, store[0].GetName())
			}
			if store[0].GetNamespace() != tc.opts.Namespace {
				t.Errorf("expected store statefulset namespace to be %s, got %s", tc.opts.Namespace, store[0].GetNamespace())
			}
			// ensure we inherit the labels from the Options struct and that the strict labels cannot be overridden
			if len(store[0].GetLabels()) != 8 {
				t.Errorf("expected store statefulset to have 8 labels, got %d", len(store[0].GetLabels()))
			}
			// ensure custom labels are set
			if store[0].GetLabels()["some-custom-label"] != someCustomLabelValue {
				t.Errorf("expected store statefulset to have label 'some-custom-label' with value 'xyz', got %s", store[0].GetLabels()["some-custom-label"])
			}
			if store[0].GetLabels()["some-other-label"] != someOtherLabelValue {
				t.Errorf("expected store statefulset to have label 'some-other-label' with value 'abc', got %s", store[0].GetLabels()["some-other-label"])
			}
			// ensure default labels are set
			expect := labelsForStoreShard(tc.opts)
			for k, v := range expect {
				if store[0].GetLabels()[k] != v {
					t.Errorf("expected store statefulset to have label %s with value %s, got %s", k, v, store[0].GetLabels()[k])
				}
			}

			storeSts := store[0].(*appsv1.StatefulSet)
			if tc.name == "test additional container" && len(storeSts.Spec.Template.Spec.Containers) != 2 {
				t.Errorf("expected query deployment to have 2 containers, got %d", len(storeSts.Spec.Template.Spec.Containers))
			}

			expectArgs := storeArgsFrom(tc.opts, 0)
			var found bool
			for _, c := range storeSts.Spec.Template.Spec.Containers {
				if c.Name == Name {
					found = true
					if c.Image != tc.opts.GetContainerImage() {
						t.Errorf("expected store statefulset to have image %s, got %s", tc.opts.GetContainerImage(), c.Image)
					}
					if len(c.Args) != len(expectArgs) {
						t.Errorf("expected store statefulset to have %d args, got %d", len(expectArgs), len(c.Args))
					}
					for i, arg := range c.Args {
						if arg != expectArgs[i] {
							t.Errorf("expected store statefulset to have arg %s, got %s", expectArgs[i], arg)
						}
					}

					if tc.name == "test additional volumemount" {
						if len(c.VolumeMounts) != 2 {
							t.Errorf("expected store statefulset to have 2 volumemount, got %d", len(c.VolumeMounts))
						}
						if c.VolumeMounts[0].Name != "data" {
							t.Errorf("expected store statefulset to have volumemount named data, got %s", c.VolumeMounts[0].Name)
						}
						if c.VolumeMounts[0].MountPath != "var/thanos/store" {
							t.Errorf("expected store statefulset to have volumemount mounted at var/thanos/store, got %s", c.VolumeMounts[0].MountPath)
						}
					}
				}
			}
			if !found {
				t.Errorf("expected store statefulset to have container named %s", Name)
			}
		})
	}
}

func TestNewStoreService(t *testing.T) {
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
		Shards: 1,
	}

	for _, tc := range []struct {
		name string
		opts Options
	}{
		{
			name: "test store service correctness",
			opts: opts,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			storeSvcs := NewStoreServices(tc.opts)
			if storeSvcs[0].GetName() != (tc.opts.Name + "-shard-0") {
				t.Errorf("expected store service name to be %s, got %s", tc.opts.Name, storeSvcs[0].GetName())
			}
			if storeSvcs[0].GetNamespace() != tc.opts.Namespace {
				t.Errorf("expected store service namespace to be %s, got %s", tc.opts.Namespace, storeSvcs[0].GetNamespace())
			}
			// ensure we inherit the labels from the Options struct and that the strict labels cannot be overridden
			if len(storeSvcs[0].GetLabels()) != 8 {
				t.Errorf("expected store service to have 8 labels, got %d", len(storeSvcs[0].GetLabels()))
			}
			// ensure custom labels are set
			if storeSvcs[0].GetLabels()["some-custom-label"] != someCustomLabelValue {
				t.Errorf("expected store service to have label 'some-custom-label' with value 'xyz', got %s", storeSvcs[0].GetLabels()["some-custom-label"])
			}
			if storeSvcs[0].GetLabels()["some-other-label"] != someOtherLabelValue {
				t.Errorf("expected store service to have label 'some-other-label' with value 'abc', got %s", storeSvcs[0].GetLabels()["some-other-label"])
			}
			// ensure default labels are set
			expect := labelsForStoreShard(tc.opts)
			for k, v := range expect {
				if storeSvcs[0].GetLabels()[k] != v {
					t.Errorf("expected store service to have label %s with value %s, got %s", k, v, storeSvcs[0].GetLabels()[k])
				}
			}

			svc := storeSvcs[0].(*corev1.Service)
			if svc.Spec.ClusterIP != "None" {
				t.Errorf("expected store service to have ClusterIP 'None', got %s", svc.Spec.ClusterIP)
			}
		})
	}
}
