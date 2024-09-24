package store

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
		},
	}

	expectService := NewStoreService(opts)
	expectStatefulSet := NewStoreStatefulSet(opts)

	objs := Build(opts)
	if len(objs) != 4 {
		t.Fatalf("expected 4 objects, got %d", len(objs))
	}

	if objs[0].GetObjectKind().GroupVersionKind().String() != "ServiceAccount" && objs[0].GetName() != Name {
		t.Errorf("expected first object to be a service account, got %v", objs[0])
	}

	if !equality.Semantic.DeepEqual(objs[0].GetLabels(), GetRequiredLabels()) {
		t.Errorf("expected service account to have labels %v, got %v", GetRequiredLabels(), objs[0].GetLabels())
	}

	if !equality.Semantic.DeepEqual(objs[2], expectStatefulSet) {
		t.Errorf("expected second object to be a statefulset, wanted \n%v\n got \n%v\n", expectStatefulSet, objs[2])
	}

	if !equality.Semantic.DeepEqual(objs[1], expectService) {
		t.Errorf("expected third object to be a service, wanted \n%v\n got \n%v\n", expectService, objs[1])
	}

	wantLabels := labelsForStoreShard(opts)
	wantLabels["some-custom-label"] = someCustomLabelValue
	wantLabels["some-other-label"] = someOtherLabelValue

	for _, obj := range []client.Object{objs[1], objs[2]} {
		if !equality.Semantic.DeepEqual(obj.GetLabels(), wantLabels) {
			t.Errorf("expected object to have labels %v, got %v", wantLabels, obj.GetLabels())
		}
	}
}

func TestNewStoreStatefulSet(t *testing.T) {
	const (
		name = "test"
		ns   = "ns"
	)

	buildDefaultOpts := func() Options {
		return Options{
			Options: manifests.Options{
				Name:      name,
				Namespace: ns,
				Image:     ptr.To("some-custom-image"),
				Labels: map[string]string{
					"some-custom-label":      someCustomLabelValue,
					"some-other-label":       someOtherLabelValue,
					"app.kubernetes.io/name": "expect-to-be-discarded",
				},
			},
		}
	}

	for _, tc := range []struct {
		name       string
		opts       func() Options
		expectName string
	}{
		{
			name: "test store stateful correctness",
			opts: func() Options {
				return buildDefaultOpts()
			},
			expectName: name,
		},
		{
			name: "test additional volumemount",
			opts: func() Options {
				opts := buildDefaultOpts()
				opts.Additional.VolumeMounts = []corev1.VolumeMount{
					{
						Name:      "test-sd",
						MountPath: "/test-sd-file",
					},
				}
				return opts
			},
			expectName: name,
		},
		{
			name: "test additional container",
			opts: func() Options {
				opts := buildDefaultOpts()
				opts.Additional.Containers = []corev1.Container{
					{
						Name:  "test-container",
						Image: "test-image:latest",
						Args:  []string{"--test-arg"},
						Env: []corev1.EnvVar{{
							Name:  "TEST_ENV",
							Value: "test",
						}},
					},
				}
				return opts
			},
			expectName: name,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			builtOpts := tc.opts()
			store := NewStoreStatefulSet(builtOpts)
			if store.GetName() != (tc.expectName) {
				t.Errorf("expected store statefulset name to be %s, got %s", tc.expectName, store.GetName())
			}
			if store.GetNamespace() != ns {
				t.Errorf("expected store statefulset namespace to be %s, got %s", ns, store.GetNamespace())
			}
			// ensure we inherit the labels from the Options struct and that the strict labels cannot be overridden
			if len(store.GetLabels()) != 9 {
				t.Errorf("expected store statefulset to have 8 labels, got %d", len(store.GetLabels()))
			}
			// ensure custom labels are set
			if store.GetLabels()["some-custom-label"] != someCustomLabelValue {
				t.Errorf("expected store statefulset to have label 'some-custom-label' with value 'xyz', got %s", store.GetLabels()["some-custom-label"])
			}
			if store.GetLabels()["some-other-label"] != someOtherLabelValue {
				t.Errorf("expected store statefulset to have label 'some-other-label' with value 'abc', got %s", store.GetLabels()["some-other-label"])
			}
			// ensure default labels are set
			expect := labelsForStoreShard(builtOpts)
			for k, v := range expect {
				if store.GetLabels()[k] != v {
					t.Errorf("expected store statefulset to have label %s with value %s, got %s", k, v, store.GetLabels()[k])
				}
			}

			storeSts := store.(*appsv1.StatefulSet)
			if tc.name == "test additional container" && len(storeSts.Spec.Template.Spec.Containers) != 2 {
				t.Errorf("expected query deployment to have 2 containers, got %d", len(storeSts.Spec.Template.Spec.Containers))
			}

			expectArgs := storeArgsFrom(builtOpts)
			var found bool
			for _, c := range storeSts.Spec.Template.Spec.Containers {
				if c.Name == Name {
					found = true
					if c.Image != builtOpts.GetContainerImage() {
						t.Errorf("expected store statefulset to have image %s, got %s", builtOpts.GetContainerImage(), c.Image)
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
	const (
		name = "test"
		ns   = "ns"
	)

	opts := Options{
		Options: manifests.Options{
			Name:      name,
			Namespace: ns,
			Image:     ptr.To("some-custom-image"),
			Labels: map[string]string{
				"some-custom-label":      someCustomLabelValue,
				"some-other-label":       someOtherLabelValue,
				"app.kubernetes.io/name": "expect-to-be-discarded",
			},
		},
	}

	for _, tc := range []struct {
		name       string
		opts       func() Options
		expectName string
	}{
		{
			name: "test store service correctness",
			opts: func() Options {
				return opts
			},
			expectName: name,
		},
		{
			name: "test store service correctness",
			opts: func() Options {
				opts.ShardName = "test-shard"
				return opts
			},
			expectName: "test-shard",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			builtOpts := tc.opts()
			storeSvc := NewStoreService(builtOpts)
			if storeSvc.GetName() != tc.expectName {
				t.Errorf("expected store service name to be %s, got %s", tc.expectName, storeSvc.GetName())
			}
			if storeSvc.GetNamespace() != ns {
				t.Errorf("expected store service namespace to be %s, got %s", ns, storeSvc.GetNamespace())
			}
			// ensure we inherit the labels from the Options struct and that the strict labels cannot be overridden
			if len(storeSvc.GetLabels()) != 9 {
				t.Errorf("expected store service to have 8 labels, got %d", len(storeSvc.GetLabels()))
			}
			// ensure custom labels are set
			if storeSvc.GetLabels()["some-custom-label"] != someCustomLabelValue {
				t.Errorf("expected store service to have label 'some-custom-label' with value 'xyz', got %s", storeSvc.GetLabels()["some-custom-label"])
			}
			if storeSvc.GetLabels()["some-other-label"] != someOtherLabelValue {
				t.Errorf("expected store service to have label 'some-other-label' with value 'abc', got %s", storeSvc.GetLabels()["some-other-label"])
			}
			// ensure default labels are set
			expect := labelsForStoreShard(builtOpts)
			for k, v := range expect {
				if storeSvc.GetLabels()[k] != v {
					t.Errorf("expected store service to have label %s with value %s, got %s", k, v, storeSvc.GetLabels()[k])
				}
			}

			svc := storeSvc.(*corev1.Service)
			if svc.Spec.ClusterIP != "None" {
				t.Errorf("expected store service to have ClusterIP 'None', got %s", svc.Spec.ClusterIP)
			}
		})
	}
}
