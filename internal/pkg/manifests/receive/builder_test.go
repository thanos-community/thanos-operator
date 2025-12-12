package receive

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/test/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
			Owner:     "any",
			Namespace: "ns",
			Image:     ptr.To("some-custom-image"),
			Labels: map[string]string{
				"some-custom-label":      someCustomLabelValue,
				"some-other-label":       someOtherLabelValue,
				"app.kubernetes.io/name": "expect-to-be-discarded",
			},
			PodDisruptionConfig: &manifests.PodDisruptionBudgetOptions{},
		},
		HashringName: "test-hashring",
	}

	objs := opts.Build()
	if len(objs) != 4 {
		t.Fatalf("expected 4 objects, got %d", len(objs))
	}

	utils.ValidateIsNamedServiceAccount(t, objs[0], opts, opts.Namespace)
	utils.ValidateObjectsEqual(t, objs[1], NewIngestorService(opts))
	utils.ValidateObjectsEqual(t, objs[2], NewIngestorStatefulSet(opts))
	utils.ValidateIsNamedPodDisruptionBudget(t, objs[3], opts, opts.Namespace, objs[1])

	wantLabels := opts.GetSelectorLabels()
	wantLabels["some-custom-label"] = someCustomLabelValue
	wantLabels["some-other-label"] = someOtherLabelValue
	utils.ValidateObjectLabelsEqual(t, wantLabels, []client.Object{objs[1], objs[2]}...)
}

func TestBuildRouter(t *testing.T) {
	opts := RouterOptions{
		Options: manifests.Options{
			Owner:     "any",
			Namespace: "ns",
			Image:     ptr.To("some-custom-image"),
			Labels: map[string]string{
				"some-custom-label":      someCustomLabelValue,
				"some-other-label":       someOtherLabelValue,
				"app.kubernetes.io/name": "expect-to-be-discarded",
			},
			PodDisruptionConfig: &manifests.PodDisruptionBudgetOptions{},
		},
	}

	objs := opts.Build()
	if len(objs) != 5 {
		t.Fatalf("expected 5 objects, got %d", len(objs))
	}

	utils.ValidateIsNamedServiceAccount(t, objs[0], opts, opts.Namespace)
	utils.ValidateObjectsEqual(t, objs[1], NewRouterService(opts))
	utils.ValidateObjectsEqual(t, objs[2], NewRouterDeployment(opts))
	utils.ValidateIsNamedPodDisruptionBudget(t, objs[4], opts, opts.Namespace, objs[2])

	wantLabels := GetRouterLabels(opts)
	wantLabels["some-custom-label"] = someCustomLabelValue
	wantLabels["some-other-label"] = someOtherLabelValue
	utils.ValidateObjectLabelsEqual(t, wantLabels, []client.Object{objs[1], objs[2]}...)
}

func TestNewIngestorStatefulSet(t *testing.T) {
	extraLabels := map[string]string{
		"some-custom-label": someCustomLabelValue,
		"some-other-label":  someOtherLabelValue,
	}

	for _, tc := range []struct {
		name string
		opts IngesterOptions
	}{
		{
			name: "test ingester statefulset correctness",
			opts: IngesterOptions{
				Options: manifests.Options{
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
					Annotations: map[string]string{
						"test": "annotation",
					},
				},
			},
		},
		{
			name: "test additional volumemount",
			opts: IngesterOptions{
				Options: manifests.Options{
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
					Annotations: map[string]string{
						"test": "annotation",
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
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
					Annotations: map[string]string{
						"test": "annotation",
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
			objectMetaLabels := GetIngesterLabels(tc.opts)
			utils.ValidateNameNamespaceAndLabels(t, ingester, tc.opts.GetGeneratedResourceName(), tc.opts.Namespace, objectMetaLabels)
			utils.ValidateHasLabels(t, ingester, tc.opts.GetSelectorLabels())
			utils.ValidateHasLabels(t, ingester, extraLabels)
			name := tc.opts.GetGeneratedResourceName()

			if ingester.Spec.ServiceName != name {
				t.Errorf("expected ingester statefulset to have serviceName %s, got %s", name, ingester.Spec.ServiceName)
			}

			if ingester.Spec.Template.Spec.ServiceAccountName != name {
				t.Errorf("expected ingester statefulset to have service account name %s, got %s", name, ingester.Spec.Template.Spec.ServiceAccountName)
			}

			if len(ingester.Spec.Template.Spec.Containers) != (len(tc.opts.Additional.Containers) + 1) {
				t.Errorf("expected ingester statefulset to have %d containers, got %d", len(tc.opts.Additional.Containers)+1, len(ingester.Spec.Template.Spec.Containers))
			}

			if ingester.Annotations["test"] != "annotation" {
				t.Errorf("expected ingester statefulset annotation test to be annotation, got %s", ingester.Annotations["test"])
			}

			expectArgs := ingestorArgsFrom(tc.opts)
			var found bool
			for _, c := range ingester.Spec.Template.Spec.Containers {
				if c.Name == IngestComponentName {
					found = true
					if c.Image != tc.opts.GetContainerImage() {
						t.Errorf("expected ingester statefulset to have image %s, got %s", tc.opts.GetContainerImage(), c.Image)
					}

					if !reflect.DeepEqual(c.Args, expectArgs) {
						t.Errorf("expected ingester statefulset to have args %v, got %v", expectArgs, c.Args)
					}

					if len(c.VolumeMounts) != len(tc.opts.Additional.VolumeMounts)+1 {
						if c.VolumeMounts[1].Name != "http-config" {
							t.Errorf("expected ingester to have volumemount named http-config, got %s", c.VolumeMounts[0].Name)
						}
						if c.VolumeMounts[1].MountPath != "/http-config" {
							t.Errorf("expected ingester to have volumemount mounted at /http-config, got %s", c.VolumeMounts[0].MountPath)
						}
					}
				}
			}
			if !found {
				t.Errorf("expected ingester to have container named %s", IngestComponentName)
			}
		})
	}
}

func TestNewRouterDeployment(t *testing.T) {
	extraLabels := map[string]string{
		"some-custom-label": someCustomLabelValue,
		"some-other-label":  someOtherLabelValue,
	}

	for _, tc := range []struct {
		name string
		opts RouterOptions
	}{
		{
			name: "test router deployment correctness",
			opts: RouterOptions{
				Options: manifests.Options{
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
					Annotations: map[string]string{
						"test": "annotation",
					},
				},
			},
		},
		{
			name: "test additional volumemount",
			opts: RouterOptions{
				Options: manifests.Options{
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
					Annotations: map[string]string{
						"test": "annotation",
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
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
					Annotations: map[string]string{
						"test": "annotation",
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
			objectMetaLabels := GetRouterLabels(tc.opts)
			utils.ValidateNameNamespaceAndLabels(t, router, tc.opts.GetGeneratedResourceName(), tc.opts.Namespace, objectMetaLabels)
			utils.ValidateHasLabels(t, router, tc.opts.GetSelectorLabels())
			utils.ValidateHasLabels(t, router, extraLabels)
			name := tc.opts.GetGeneratedResourceName()

			if router.Spec.Template.Spec.ServiceAccountName != name {
				t.Errorf("expected deployment to use service account %s, got %s", name, router.Spec.Template.Spec.ServiceAccountName)
			}
			if len(router.Spec.Template.Spec.Containers) != (len(tc.opts.Additional.Containers) + 1) {
				t.Errorf("expected deployment to have %d containers, got %d", len(tc.opts.Additional.Containers)+1, len(router.Spec.Template.Spec.Containers))
			}

			if router.Annotations["test"] != "annotation" {
				t.Errorf("expected router deployment annotation test to be annotation, got %s", router.Annotations["test"])
			}

			expectArgs := routerArgsFrom(tc.opts)
			var found bool
			for _, c := range router.Spec.Template.Spec.Containers {
				if c.Name == RouterComponentName {
					found = true
					if c.Image != tc.opts.GetContainerImage() {
						t.Errorf("expected router deployment to have image %s, got %s", tc.opts.GetContainerImage(), c.Image)
					}

					if !reflect.DeepEqual(c.Args, expectArgs) {
						t.Errorf("expected router deployment to have args %v, got %v", expectArgs, c.Args)
					}

					if len(c.VolumeMounts) != len(tc.opts.Additional.VolumeMounts)+1 {
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
	extraLabels := map[string]string{
		"some-custom-label": someCustomLabelValue,
		"some-other-label":  someOtherLabelValue,
	}

	opts := IngesterOptions{
		Options: manifests.Options{
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
			objectMetaLabels := GetIngesterLabels(tc.opts)
			utils.ValidateNameNamespaceAndLabels(t, ingester, tc.opts.GetGeneratedResourceName(), opts.Namespace, objectMetaLabels)
			utils.ValidateHasLabels(t, ingester, extraLabels)
			utils.ValidateHasLabels(t, ingester, tc.opts.GetSelectorLabels())

			if ingester.Spec.ClusterIP != corev1.ClusterIPNone {
				t.Errorf("expected store service to have ClusterIP 'None', got %s", ingester.Spec.ClusterIP)
			}
		})
	}
}

func TestNewRouterService(t *testing.T) {
	extraLabels := map[string]string{
		"some-custom-label": someCustomLabelValue,
		"some-other-label":  someOtherLabelValue,
	}
	opts := RouterOptions{
		Options: manifests.Options{
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
			objectMetaLabels := GetRouterLabels(opts)
			utils.ValidateNameNamespaceAndLabels(t, router, opts.GetGeneratedResourceName(), opts.Namespace, objectMetaLabels)
			utils.ValidateHasLabels(t, router, extraLabels)
			utils.ValidateHasLabels(t, router, opts.GetSelectorLabels())
		})
	}
}

func TestKubeResourceSyncFeatureGate(t *testing.T) {
	tests := []struct {
		name                    string
		featureGateEnabled      bool
		expectedContainerCount  int
		expectedVolumeType      string
		expectedSidecarPresent  bool
	}{
		{
			name:                    "kube-resource-sync disabled (default)",
			featureGateEnabled:      false,
			expectedContainerCount:  1,
			expectedVolumeType:      "ConfigMap",
			expectedSidecarPresent:  false,
		},
		{
			name:                    "kube-resource-sync enabled",
			featureGateEnabled:      true,
			expectedContainerCount:  2,
			expectedVolumeType:      "EmptyDir",
			expectedSidecarPresent:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := RouterOptions{
				Options: manifests.Options{
					Owner:     "test-receive",
					Namespace: "test-ns",
					Image:     ptr.To("quay.io/thanos/thanos:latest"),
				},
			}

			if tt.featureGateEnabled {
				opts.FeatureGates = &v1alpha1.FeatureGates{
					KubeResourceSyncConfig: &v1alpha1.KubeResourceSyncConfig{
						Enable: ptr.To(true),
					},
				}
			}

			deployment := NewRouterDeployment(opts)

			// Test container count
			if len(deployment.Spec.Template.Spec.Containers) != tt.expectedContainerCount {
				t.Errorf("expected %d containers, got %d", tt.expectedContainerCount, len(deployment.Spec.Template.Spec.Containers))
			}

			// Test volume configuration
			if len(deployment.Spec.Template.Spec.Volumes) != 1 {
				t.Errorf("expected 1 volume, got %d", len(deployment.Spec.Template.Spec.Volumes))
			}

			volume := deployment.Spec.Template.Spec.Volumes[0]
			if volume.Name != hashringVolumeName {
				t.Errorf("expected volume name %s, got %s", hashringVolumeName, volume.Name)
			}

			if tt.expectedVolumeType == "ConfigMap" {
				if volume.VolumeSource.ConfigMap == nil {
					t.Error("expected ConfigMap volume source but got nil")
				}
				if volume.VolumeSource.EmptyDir != nil {
					t.Error("expected no EmptyDir volume source but found one")
				}
			} else if tt.expectedVolumeType == "EmptyDir" {
				if volume.VolumeSource.EmptyDir == nil {
					t.Error("expected EmptyDir volume source but got nil")
				}
				if volume.VolumeSource.ConfigMap != nil {
					t.Error("expected no ConfigMap volume source but found one")
				}
			}

			// Test sidecar container presence
			sidecarPresent := false
			thanosRouterPresent := false
			
			for _, container := range deployment.Spec.Template.Spec.Containers {
				if container.Name == kubeResourceSyncContainerName {
					sidecarPresent = true
					
					// Verify sidecar configuration
					if !tt.expectedSidecarPresent {
						t.Error("kube-resource-sync sidecar should not be present when feature gate is disabled")
					}
					
					// Check sidecar arguments
					expectedArgs := []string{
						"--configmap-name=" + opts.GetGeneratedResourceName(),
						"--configmap-namespace=" + opts.Namespace,
						"--configmap-key=" + HashringConfigKey,
						"--output-path=" + hashringMountPath + "/" + HashringConfigKey,
					}
					
					if !reflect.DeepEqual(container.Args, expectedArgs) {
						t.Errorf("expected sidecar args %v, got %v", expectedArgs, container.Args)
					}
					
					// Check volume mount
					if len(container.VolumeMounts) != 1 {
						t.Errorf("expected 1 volume mount for sidecar, got %d", len(container.VolumeMounts))
					} else {
						mount := container.VolumeMounts[0]
						if mount.Name != hashringVolumeName {
							t.Errorf("expected volume mount name %s, got %s", hashringVolumeName, mount.Name)
						}
						if mount.MountPath != hashringMountPath {
							t.Errorf("expected volume mount path %s, got %s", hashringMountPath, mount.MountPath)
						}
					}
				} else if container.Name == RouterComponentName {
					thanosRouterPresent = true
				}
			}

			if tt.expectedSidecarPresent && !sidecarPresent {
				t.Error("expected kube-resource-sync sidecar to be present when feature gate is enabled")
			}

			if !thanosRouterPresent {
				t.Error("Thanos router container should always be present")
			}
		})
	}
}

var update = flag.Bool("update", false, "update golden files")

func TestKubeResourceSyncFeatureGate_Golden(t *testing.T) {
	tests := []struct {
		name             string
		featureGateEnabled bool
	}{
		{
			name:               "router-deployment-without-kube-resource-sync",
			featureGateEnabled: false,
		},
		{
			name:               "router-deployment-with-kube-resource-sync",
			featureGateEnabled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := RouterOptions{
				Options: manifests.Options{
					Owner:     "test-receive",
					Namespace: "test-ns",
					Image:     ptr.To("quay.io/thanos/thanos:v0.39.0"),
					Version:   ptr.To("v0.39.0"),
					Labels: map[string]string{
						"app.kubernetes.io/name": "thanos-receive",
					},
					Annotations: map[string]string{
						"test": "annotation",
					},
				},
				HashringConfig: `[{"hashring": "test", "tenants": ["tenant1"], "endpoints": ["http://test:10901"]}]`,
			}

			if tt.featureGateEnabled {
				opts.FeatureGates = &v1alpha1.FeatureGates{
					KubeResourceSyncConfig: &v1alpha1.KubeResourceSyncConfig{
						Enable: ptr.To(true),
						Image:  ptr.To("ghcr.io/philipgough/kube-resource-sync:v0.1.0"),
					},
				}
			}

			deployment := NewRouterDeployment(opts)
			
			// Remove runtime fields that can vary between test runs
			deployment.CreationTimestamp = metav1.Time{}
			for i := range deployment.Spec.Template.Spec.Containers {
				if deployment.Spec.Template.Spec.Containers[i].TerminationMessagePath != "" {
					deployment.Spec.Template.Spec.Containers[i].TerminationMessagePath = ""
				}
				if deployment.Spec.Template.Spec.Containers[i].TerminationMessagePolicy != "" {
					deployment.Spec.Template.Spec.Containers[i].TerminationMessagePolicy = ""
				}
			}

			// Create golden file path
			goldenFilePath := filepath.Join("testdata", tt.name+".golden.json")

			if *update {
				// Update golden file
				bytes, err := json.MarshalIndent(deployment, "", "  ")
				require.NoError(t, err)

				err = os.MkdirAll(filepath.Dir(goldenFilePath), 0755)
				require.NoError(t, err)

				err = os.WriteFile(goldenFilePath, bytes, 0644)
				require.NoError(t, err)
			}

			// Read golden file
			bytes, err := os.ReadFile(goldenFilePath)
			require.NoError(t, err)

			var expected appsv1.Deployment
			err = json.Unmarshal(bytes, &expected)
			require.NoError(t, err)

			// Compare the objects
			assert.Equal(t, expected, *deployment)
		})
	}
}
