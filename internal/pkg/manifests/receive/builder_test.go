package receive

import (
	"fmt"
	"testing"

	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"

	"github.com/go-logr/logr/testr"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/diff"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
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

	expectService := NewIngestorService(opts)
	expectStatefulSet := NewIngestorStatefulSet(opts)

	objs := BuildIngester(opts)
	if len(objs) != 3 {
		t.Fatalf("expected 3 objects, got %d", len(objs))
	}

	if objs[0].GetObjectKind().GroupVersionKind().String() != "ServiceAccount" && objs[0].GetName() != Name {
		t.Errorf("expected first object to be a service account, got %v", objs[0])
	}

	if !equality.Semantic.DeepEqual(objs[0].GetLabels(), GetRequiredLabels()) {
		t.Errorf("expected service account to have labels %v, got %v", GetRequiredLabels(), objs[0].GetLabels())
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

	for _, obj := range []client.Object{objs[1], objs[2]} {
		if !equality.Semantic.DeepEqual(obj.GetLabels(), wantLabels) {
			t.Errorf("expected object to have labels %v, got %v", wantLabels, obj.GetLabels())
		}
	}
}

func TestBuildRouter(t *testing.T) {
	opts := RouterOptions{
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

	expectService := NewRouterService(opts)
	expectDeployment := NewRouterDeployment(opts)

	objs := BuildRouter(opts)
	if len(objs) != 3 {
		t.Fatalf("expected 3 objects, got %d", len(objs))
	}

	if objs[0].GetObjectKind().GroupVersionKind().String() != "ServiceAccount" && objs[0].GetName() != Name {
		t.Errorf("expected first object to be a service account, got %v", objs[0])
	}

	if !equality.Semantic.DeepEqual(objs[0].GetLabels(), GetRequiredLabels()) {
		t.Errorf("expected service account to have labels %v, got %v", GetRequiredLabels(), objs[0].GetLabels())
	}

	if !equality.Semantic.DeepEqual(objs[1], expectService) {
		t.Errorf("expected first object to be a service, wanted \n%v\n got \n%v\n", expectService, objs[1])
	}

	if !equality.Semantic.DeepEqual(objs[2], expectDeployment) {
		t.Errorf("expected third object to be a deployment, wanted \n%v\n got \n%v\n", expectDeployment, objs[2])
	}

	if expectDeployment.Spec.Template.Spec.ServiceAccountName != Name {
		t.Errorf("expected deployment to have service account %s, got %s", Name, expectDeployment.Spec.Template.Spec.ServiceAccountName)
	}

	wantLabels := labelsForRouter(opts.Options)
	wantLabels["some-custom-label"] = someCustomLabelValue
	wantLabels["some-other-label"] = someOtherLabelValue

	for _, obj := range []client.Object{objs[1], objs[2]} {
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
	for _, tc := range []struct {
		name string
		opts IngesterOptions
	}{
		{
			name: "test ingester statefulset correctness",
			opts: IngesterOptions{
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
		{
			name: "test additional volumemount",
			opts: IngesterOptions{
				Options: manifests.Options{
					Name:      "test",
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
					Additional: manifests.Additional{
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "http-config",
								MountPath: "/http-config",
							},
						},
					},
				},
			},
		},
		{
			name: "test additional container",
			opts: IngesterOptions{
				Options: manifests.Options{
					Name:      "test",
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
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
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ingester := NewIngestorStatefulSet(tc.opts)
			if ingester.GetName() != tc.opts.Name {
				t.Errorf("expected ingester statefulset name to be %s, got %s", tc.opts.Name, ingester.GetName())
			}
			if ingester.GetNamespace() != tc.opts.Namespace {
				t.Errorf("expected ingester statefulset namespace to be %s, got %s", tc.opts.Namespace, ingester.GetNamespace())
			}
			// ensure we inherit the labels from the Options struct and that the strict labels cannot be overridden
			if len(ingester.GetLabels()) != 9 {
				t.Errorf("expected ingester statefulset to have 8 labels, got %d", len(ingester.GetLabels()))
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

			if tc.name == "test additional container" && len(ingester.Spec.Template.Spec.Containers) != 2 {
				t.Errorf("expected ingester deployment to have 2 containers, got %d", len(ingester.Spec.Template.Spec.Containers))
			}

			expectArgs := ingestorArgsFrom(tc.opts)
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

					if tc.name == "test additional volumemount" {
						if len(c.VolumeMounts) != 2 {
							t.Errorf("expected ingester deployment to have 2 volumemount, got %d", len(c.VolumeMounts))
						}
						if c.VolumeMounts[1].Name != "http-config" {
							t.Errorf("expected ingester deployment to have volumemount named http-config, got %s", c.VolumeMounts[0].Name)
						}
						if c.VolumeMounts[1].MountPath != "/http-config" {
							t.Errorf("expected ingester deployment to have volumemount mounted at /http-config, got %s", c.VolumeMounts[0].MountPath)
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

func TestNewRouterDeployment(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts RouterOptions
	}{
		{
			name: "test router deployment correctness",
			opts: RouterOptions{
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
		{
			name: "test additional volumemount",
			opts: RouterOptions{
				Options: manifests.Options{
					Name:      "test",
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
					Additional: manifests.Additional{
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "http-config",
								MountPath: "/http-config",
							},
						},
					},
				},
			},
		},
		{
			name: "test additional container",
			opts: RouterOptions{
				Options: manifests.Options{
					Name:      "test",
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
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
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := NewRouterDeployment(tc.opts)
			if router.GetName() != tc.opts.Name {
				t.Errorf("expected router deployment name to be %s, got %s", tc.opts.Name, router.GetName())
			}
			if router.GetNamespace() != tc.opts.Namespace {
				t.Errorf("expected router deployment namespace to be %s, got %s", tc.opts.Namespace, router.GetNamespace())
			}
			// ensure we inherit the labels from the Options struct and that the strict labels cannot be overridden
			if len(router.GetLabels()) != 7 {
				t.Errorf("expected router deployment to have 7 labels, got %d", len(router.GetLabels()))
			}
			// ensure custom labels are set
			if router.GetLabels()["some-custom-label"] != someCustomLabelValue {
				t.Errorf("expected router deployment to have label 'some-custom-label' with value 'xyz', got %s", router.GetLabels()["some-custom-label"])
			}
			if router.GetLabels()["some-other-label"] != someOtherLabelValue {
				t.Errorf("expected router deployment to have label 'some-other-label' with value 'abc', got %s", router.GetLabels()["some-other-label"])
			}
			// ensure default labels are set
			expect := labelsForRouter(tc.opts.Options)
			for k, v := range expect {
				if router.GetLabels()[k] != v {
					t.Errorf("expected router deployment to have label %s with value %s, got %s", k, v, router.GetLabels()[k])
				}
			}

			if tc.name == "test additional container" && len(router.Spec.Template.Spec.Containers) != 2 {
				t.Errorf("expected router deployment to have 2 containers, got %d", len(router.Spec.Template.Spec.Containers))
			}

			expectArgs := routerArgsFrom(tc.opts)
			var found bool
			for _, c := range router.Spec.Template.Spec.Containers {
				if c.Name == RouterComponentName {
					found = true
					if c.Image != tc.opts.GetContainerImage() {
						t.Errorf("expected router deployment to have image %s, got %s", tc.opts.GetContainerImage(), c.Image)
					}
					if len(c.Args) != len(expectArgs) {
						t.Errorf("expected router deployment to have %d args, got %d", len(expectArgs), len(c.Args))
					}
					for i, arg := range c.Args {
						if arg != expectArgs[i] {
							t.Errorf("expected router deployment to have arg %s, got %s", expectArgs[i], arg)
						}
					}

					if tc.name == "test additional volumemount" {
						if len(c.VolumeMounts) != 2 {
							t.Errorf("expected router deployment to have 2 volumemount, got %d", len(c.VolumeMounts))
						}
						if c.VolumeMounts[1].Name != "http-config" {
							t.Errorf("expected router deployment to have volumemount named http-config, got %s", c.VolumeMounts[0].Name)
						}
						if c.VolumeMounts[1].MountPath != "/http-config" {
							t.Errorf("expected router deployment to have volumemount mounted at /http-config, got %s", c.VolumeMounts[0].MountPath)
						}
					}
				}
			}
			if !found {
				t.Errorf("expected router deployment to have container named %s", RouterComponentName)
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
			ingester := NewIngestorService(tc.opts)
			if ingester.GetName() != tc.opts.Name {
				t.Errorf("expected ingester service name to be %s, got %s", tc.opts.Name, ingester.GetName())
			}
			if ingester.GetNamespace() != tc.opts.Namespace {
				t.Errorf("expected ingester service namespace to be %s, got %s", tc.opts.Namespace, ingester.GetNamespace())
			}
			// ensure we inherit the labels from the Options struct and that the strict labels cannot be overridden
			if len(ingester.GetLabels()) != 9 {
				t.Errorf("expected ingester service to have 8 labels, got %d", len(ingester.GetLabels()))
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

func TestNewRouterService(t *testing.T) {
	opts := RouterOptions{
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
		opts RouterOptions
	}{
		{
			name: "test router service correctness",
			opts: opts,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := NewRouterService(tc.opts)
			if router.GetName() != tc.opts.Name {
				t.Errorf("expected router service name to be %s, got %s", tc.opts.Name, router.GetName())
			}
			if router.GetNamespace() != tc.opts.Namespace {
				t.Errorf("expected router service namespace to be %s, got %s", tc.opts.Namespace, router.GetNamespace())
			}
			// ensure we inherit the labels from the Options struct and that the strict labels cannot be overridden
			if len(router.GetLabels()) != 7 {
				t.Errorf("expected router service to have 7 labels, got %d", len(router.GetLabels()))
			}
			// ensure custom labels are set
			if router.GetLabels()["some-custom-label"] != someCustomLabelValue {
				t.Errorf("expected router service to have label 'some-custom-label' with value 'xyz', got %s", router.GetLabels()["some-custom-label"])
			}
			if router.GetLabels()["some-other-label"] != someOtherLabelValue {
				t.Errorf("expected router service to have label 'some-other-label' with value 'abc', got %s", router.GetLabels()["some-other-label"])
			}
			// ensure default labels are set
			expect := labelsForRouter(tc.opts.Options)
			for k, v := range expect {
				if router.GetLabels()[k] != v {
					t.Errorf("expected router service to have label %s with value %s, got %s", k, v, router.GetLabels()[k])
				}
			}
		})
	}
}

func TestBuildHashrings(t *testing.T) {
	logger := testr.New(t)
	baseOptions := manifests.Options{
		Name:      "test",
		Namespace: "test",
		Replicas:  3,
	}

	for _, tc := range []struct {
		name        string
		passedState Hashrings
		opts        func() HashringOptions
		expect      client.Object
	}{
		{
			name:        "test result when no state is passed",
			passedState: nil,
			opts: func() HashringOptions {
				return HashringOptions{
					Options: baseOptions,
				}
			},
			expect: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					Labels:    labelsForRouter(baseOptions),
				},
				Data: map[string]string{
					HashringConfigKey: EmptyHashringConfig,
				},
			},
		},
		{
			name:        "test result when no previous state is passed but we have new state with missing owner reference",
			passedState: nil,
			opts: func() HashringOptions {
				return HashringOptions{
					Options: baseOptions,
					HashringSettings: map[string]HashringMeta{
						"test": {
							DesiredReplicasReplicas: 2,
							AssociatedEndpointSlices: discoveryv1.EndpointSliceList{
								Items: []discoveryv1.EndpointSlice{
									{
										Endpoints: []discoveryv1.Endpoint{
											{
												Addresses: []string{"a"},
												Conditions: discoveryv1.EndpointConditions{
													Ready: ptr.To(false),
												},
											},
											{
												Addresses: []string{"b"},
												Conditions: discoveryv1.EndpointConditions{
													Ready: ptr.To(false),
												},
											},
										},
									},
								},
							},
						},
					},
				}
			},
			expect: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					Labels:    labelsForRouter(baseOptions),
				},
				Data: map[string]string{
					HashringConfigKey: EmptyHashringConfig,
				},
			},
		},
		{
			name:        "test result when no previous state is passed but we have new state with mismatched owner reference",
			passedState: nil,
			opts: func() HashringOptions {
				return HashringOptions{
					Options: baseOptions,
					HashringSettings: map[string]HashringMeta{
						"test": {
							DesiredReplicasReplicas: 2,
							AssociatedEndpointSlices: discoveryv1.EndpointSliceList{
								Items: []discoveryv1.EndpointSlice{
									{
										ObjectMeta: metav1.ObjectMeta{
											OwnerReferences: []metav1.OwnerReference{
												{
													Name: "some-other-resource",
													Kind: "Service",
												},
											},
										},
										Endpoints: []discoveryv1.Endpoint{
											{
												Addresses: []string{"a"},
												Hostname:  ptr.To("a"),
												Conditions: discoveryv1.EndpointConditions{
													Ready: ptr.To(false),
												},
											},
											{
												Addresses: []string{"b"},
												Hostname:  ptr.To("b"),
												Conditions: discoveryv1.EndpointConditions{
													Ready: ptr.To(false),
												},
											},
										},
									},
								},
							},
						},
					},
				}
			},
			expect: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					Labels:    labelsForRouter(baseOptions),
				},
				Data: map[string]string{
					HashringConfigKey: EmptyHashringConfig,
				},
			},
		},
		{
			name:        "test result when no previous state is passed but we have new state which is not ready",
			passedState: nil,
			opts: func() HashringOptions {
				return HashringOptions{
					Options: baseOptions,
					HashringSettings: map[string]HashringMeta{
						"test": {
							DesiredReplicasReplicas: 2,
							AssociatedEndpointSlices: discoveryv1.EndpointSliceList{
								Items: []discoveryv1.EndpointSlice{
									{
										ObjectMeta: metav1.ObjectMeta{
											OwnerReferences: []metav1.OwnerReference{
												{
													Name: "test",
													Kind: "Service",
												},
											},
										},
										Endpoints: []discoveryv1.Endpoint{
											{
												Addresses: []string{"a"},
												Hostname:  ptr.To("a"),
												Conditions: discoveryv1.EndpointConditions{
													Ready: ptr.To(false),
												},
											},
											{
												Addresses: []string{"b"},
												Hostname:  ptr.To("b"),
												Conditions: discoveryv1.EndpointConditions{
													Ready: ptr.To(false),
												},
											},
										},
									},
								},
							},
						},
					},
				}
			},
			expect: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					Labels:    labelsForRouter(baseOptions),
				},
				Data: map[string]string{
					HashringConfigKey: EmptyHashringConfig,
				},
			},
		},
		{
			name:        "test result when no previous state is passed but we have new state which is ready",
			passedState: nil,
			opts: func() HashringOptions {
				return HashringOptions{
					Options: baseOptions,
					HashringSettings: map[string]HashringMeta{
						"test": {
							DesiredReplicasReplicas: 2,
							AssociatedEndpointSlices: discoveryv1.EndpointSliceList{
								Items: []discoveryv1.EndpointSlice{
									{
										ObjectMeta: metav1.ObjectMeta{
											Namespace: "test",
											OwnerReferences: []metav1.OwnerReference{
												{
													Name: "test",
													Kind: "Service",
												},
											},
										},
										Endpoints: []discoveryv1.Endpoint{
											{
												Addresses: []string{"a"},
												Hostname:  ptr.To("a"),
												Conditions: discoveryv1.EndpointConditions{
													Ready: ptr.To(true),
												},
											},
											{
												Addresses: []string{"b"},
												Hostname:  ptr.To("b"),
												Conditions: discoveryv1.EndpointConditions{
													Ready: ptr.To(true),
												},
											},
										},
									},
								},
							},
						},
					},
				}
			},
			expect: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					Labels:    labelsForRouter(baseOptions),
				},
				Data: map[string]string{
					HashringConfigKey: `[
    {
        "endpoints": [
            {
                "address": "a.test.test.svc.cluster.local:10901",
                "az": ""
            },
            {
                "address": "b.test.test.svc.cluster.local:10901",
                "az": ""
            }
        ]
    }
]`,
				},
			},
		},
		{
			name:        "test result when tenants are set",
			passedState: nil,
			opts: func() HashringOptions {
				return HashringOptions{
					Options: baseOptions,
					HashringSettings: map[string]HashringMeta{
						"test": {
							OriginalName:            "test",
							DesiredReplicasReplicas: 2,
							Tenants:                 []string{"foobar"},
							AssociatedEndpointSlices: discoveryv1.EndpointSliceList{
								Items: []discoveryv1.EndpointSlice{
									{
										ObjectMeta: metav1.ObjectMeta{
											Namespace: "test",
											OwnerReferences: []metav1.OwnerReference{
												{
													Name: "test",
													Kind: "Service",
												},
											},
										},
										Endpoints: []discoveryv1.Endpoint{
											{
												Addresses: []string{"a"},
												Hostname:  ptr.To("a"),
												Conditions: discoveryv1.EndpointConditions{
													Ready: ptr.To(true),
												},
											},
											{
												Addresses: []string{"b"},
												Hostname:  ptr.To("b"),
												Conditions: discoveryv1.EndpointConditions{
													Ready: ptr.To(true),
												},
											},
										},
									},
								},
							},
						},
					},
				}
			},
			expect: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					Labels:    labelsForRouter(baseOptions),
				},
				Data: map[string]string{
					HashringConfigKey: `[
    {
        "hashring": "test",
        "tenants": [
            "foobar"
        ],
        "endpoints": [
            {
                "address": "a.test.test.svc.cluster.local:10901",
                "az": ""
            },
            {
                "address": "b.test.test.svc.cluster.local:10901",
                "az": ""
            }
        ]
    }
]`,
				},
			},
		},
		{
			name:        "test result with multiple hashrings where tenants are set",
			passedState: nil,
			opts: func() HashringOptions {
				return HashringOptions{
					Options: baseOptions,
					HashringSettings: map[string]HashringMeta{
						"a": {
							OriginalName:            "a",
							DesiredReplicasReplicas: 2,
							Tenants:                 []string{"foobar"},
							Priority:                1,
							AssociatedEndpointSlices: discoveryv1.EndpointSliceList{
								Items: []discoveryv1.EndpointSlice{
									{
										ObjectMeta: metav1.ObjectMeta{
											Namespace: "test",
											OwnerReferences: []metav1.OwnerReference{
												{
													Name: "a",
													Kind: "Service",
												},
											},
										},
										Endpoints: []discoveryv1.Endpoint{
											{
												Addresses: []string{"a"},
												Hostname:  ptr.To("a"),
												Conditions: discoveryv1.EndpointConditions{
													Ready: ptr.To(true),
												},
											},
											{
												Addresses: []string{"a1"},
												Hostname:  ptr.To("a1"),
												Conditions: discoveryv1.EndpointConditions{
													Ready: ptr.To(true),
												},
											},
										},
									},
								},
							},
						},
						"b": {
							OriginalName:            "b",
							DesiredReplicasReplicas: 2,
							TenantMatcherType:       TenantMatcherGlob,
							Tenants:                 []string{"baz*"},
							AssociatedEndpointSlices: discoveryv1.EndpointSliceList{
								Items: []discoveryv1.EndpointSlice{
									{
										ObjectMeta: metav1.ObjectMeta{
											Namespace: "test",
											OwnerReferences: []metav1.OwnerReference{
												{
													Name: "b",
													Kind: "Service",
												},
											},
										},
										Endpoints: []discoveryv1.Endpoint{
											{
												Addresses: []string{"b"},
												Hostname:  ptr.To("b"),
												Conditions: discoveryv1.EndpointConditions{
													Ready: ptr.To(true),
												},
											},
											{
												Addresses: []string{"b1"},
												Hostname:  ptr.To("b1"),
												Conditions: discoveryv1.EndpointConditions{
													Ready: ptr.To(true),
												},
											},
										},
									},
								},
							},
						},
					},
				}
			},
			expect: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					Labels:    labelsForRouter(baseOptions),
				},
				Data: map[string]string{
					HashringConfigKey: `[
    {
        "hashring": "a",
        "tenants": [
            "foobar"
        ],
        "endpoints": [
            {
                "address": "a.a.test.svc.cluster.local:10901",
                "az": ""
            },
            {
                "address": "a1.a.test.svc.cluster.local:10901",
                "az": ""
            }
        ]
    },
    {
        "hashring": "b",
        "tenants": [
            "baz*"
        ],
        "tenant_matcher_type": "glob",
        "endpoints": [
            {
                "address": "b.b.test.svc.cluster.local:10901",
                "az": ""
            },
            {
                "address": "b1.b.test.svc.cluster.local:10901",
                "az": ""
            }
        ]
    }
]`,
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cm := &corev1.ConfigMap{}
			if tc.passedState != nil {
				conf, err := tc.passedState.toJson()
				if err != nil {
					t.Fatalf("failed to marshal passed state: %v", err)
				}
				cm.Data = map[string]string{
					HashringConfigKey: conf,
				}
			}

			got, _ := BuildHashrings(logger, cm, tc.opts())
			if got == nil {
				t.Errorf("expected BuildHashrings to return a ConfigMap, got nil")
			}

			if !equality.Semantic.DeepEqual(got, tc.expect) {
				fmt.Println(diff.ObjectDiff(tc.expect, got))
				t.Errorf("expected BuildHashrings to return a ConfigMap with data \n%v, \ngot \n%v", tc.expect, got)
			}
		})
	}
}
