package queryfrontend

import (
	"testing"

	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/utils/ptr"
)

const (
	someCustomLabelValue string = "xyz"
	someOtherLabelValue  string = "abc"
)

func TestBuildQueryFrontend(t *testing.T) {
	opts := QueryFrontendOptions{
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
		QueryService:         "thanos-query",
		LogQueriesLongerThan: "5s",
		CompressResponses:    true,
		RangeSplitInterval:   "1h",
		LabelsSplitInterval:  "30m",
		RangeMaxRetries:      5,
		LabelsMaxRetries:     3,
	}

	expectSA := manifests.BuildServiceAccount(opts.Options)
	expectService := NewQueryFrontendService(opts)
	expectDeployment := NewQueryFrontendDeployment(opts)
	expectConfigMap := NewQueryFrontendInMemoryConfigMap(opts)

	objs := BuildQueryFrontend(opts)
	if len(objs) != 4 {
		t.Fatalf("expected 4 objects, got %d", len(objs))
	}

	if !equality.Semantic.DeepEqual(objs[0], expectSA) {
		t.Errorf("expected first object to be a service account, got %v", objs[0])
	}

	if !equality.Semantic.DeepEqual(objs[1], expectDeployment) {
		t.Errorf("expected second object to be a deployment, got %v", objs[1])
	}

	if !equality.Semantic.DeepEqual(objs[2], expectService) {
		t.Errorf("expected third object to be a service, got %v", objs[2])
	}

	if !equality.Semantic.DeepEqual(objs[3], expectConfigMap) {
		t.Errorf("expected fourth object to be a configmap, got %v", objs[3])
	}

	wantLabels := labelsForQueryFrontend(opts)
	wantLabels["some-custom-label"] = someCustomLabelValue
	wantLabels["some-other-label"] = someOtherLabelValue

	for _, obj := range objs {
		if !equality.Semantic.DeepEqual(obj.GetLabels(), wantLabels) {
			t.Errorf("expected object to have labels %v, got %v", wantLabels, obj.GetLabels())
		}
	}
}

func TestNewQueryFrontendDeployment(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts QueryFrontendOptions
	}{
		{
			name: "test query frontend deployment correctness",
			opts: QueryFrontendOptions{
				Options: manifests.Options{
					Name:      "test",
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
					Replicas: 2,
				}.ApplyDefaults(),
				QueryService:         "thanos-query",
				LogQueriesLongerThan: "5s",
				CompressResponses:    true,
				RangeSplitInterval:   "1h",
				LabelsSplitInterval:  "30m",
				RangeMaxRetries:      5,
				LabelsMaxRetries:     3,
			},
		},
		{
			name: "test additional volumemount",
			opts: QueryFrontendOptions{
				Options: manifests.Options{
					Name:      "test",
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
					Replicas: 2,
				}.ApplyDefaults(),
				QueryService:         "thanos-query",
				LogQueriesLongerThan: "5s",
				CompressResponses:    true,
				RangeSplitInterval:   "1h",
				LabelsSplitInterval:  "30m",
				RangeMaxRetries:      5,
				LabelsMaxRetries:     3,
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
			opts: QueryFrontendOptions{
				Options: manifests.Options{
					Name:      "test",
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
					Replicas: 2,
				}.ApplyDefaults(),
				QueryService:         "thanos-query",
				LogQueriesLongerThan: "5s",
				CompressResponses:    true,
				RangeSplitInterval:   "1h",
				LabelsSplitInterval:  "30m",
				RangeMaxRetries:      5,
				LabelsMaxRetries:     3,
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
			tc.opts.Options = tc.opts.ApplyDefaults()
			deployment := NewQueryFrontendDeployment(tc.opts)

			if deployment.GetName() != tc.opts.Name {
				t.Errorf("expected deployment name to be %s, got %s", tc.opts.Name, deployment.GetName())
			}

			if deployment.GetNamespace() != tc.opts.Namespace {
				t.Errorf("expected deployment namespace to be %s, got %s", tc.opts.Namespace, deployment.GetNamespace())
			}

			if *deployment.Spec.Replicas != tc.opts.Replicas {
				t.Errorf("expected deployment replicas to be %d, got %d", tc.opts.Replicas, *deployment.Spec.Replicas)
			}

			// Check labels
			wantLabels := labelsForQueryFrontend(tc.opts)
			wantLabels["some-custom-label"] = someCustomLabelValue
			wantLabels["some-other-label"] = someOtherLabelValue

			if !equality.Semantic.DeepEqual(deployment.GetLabels(), wantLabels) {
				t.Errorf("expected deployment to have labels %v, got %v", wantLabels, deployment.GetLabels())
			}

			// Check containers
			containers := deployment.Spec.Template.Spec.Containers
			if tc.name == "test additional container" && len(containers) != 2 {
				t.Errorf("expected deployment to have 2 containers, got %d", len(containers))
			}

			var found bool
			for _, c := range containers {
				if c.Name == Name {
					found = true
					if c.Image != tc.opts.GetContainerImage() {
						t.Errorf("expected container image to be %s, got %s", tc.opts.GetContainerImage(), c.Image)
					}

					expectedArgs := queryFrontendArgs(tc.opts)
					if !equality.Semantic.DeepEqual(c.Args, expectedArgs) {
						t.Errorf("expected container args to be %v, got %v", expectedArgs, c.Args)
					}

					if len(c.Ports) != 1 || c.Ports[0].ContainerPort != HTTPPort {
						t.Errorf("expected container to have 1 port (%d), got %v", HTTPPort, c.Ports)
					}

					if len(c.Env) != 1 || c.Env[0].Name != "CACHE_CONFIG" {
						t.Errorf("expected container to have 1 env var (CACHE_CONFIG), got %v", c.Env)
					}

					if tc.name == "test additional volumemount" {
						if len(c.VolumeMounts) != 1 {
							t.Errorf("expected container to have 1 volumemount, got %d", len(c.VolumeMounts))
						}
						if c.VolumeMounts[0].Name != "test-sd" {
							t.Errorf("expected volumemount named test-sd, got %s", c.VolumeMounts[0].Name)
						}
						if c.VolumeMounts[0].MountPath != "/test-sd-file" {
							t.Errorf("expected volumemount mounted at /test-sd-file, got %s", c.VolumeMounts[0].MountPath)
						}
					}
				}
			}
			if !found {
				t.Errorf("expected deployment to have container named %s", Name)
			}
		})
	}
}

func TestNewQueryFrontendService(t *testing.T) {
	opts := QueryFrontendOptions{
		Options: manifests.Options{
			Name:      "test",
			Namespace: "ns",
			Labels: map[string]string{
				"some-custom-label":      someCustomLabelValue,
				"some-other-label":       someOtherLabelValue,
				"app.kubernetes.io/name": "expect-to-be-discarded",
			},
		}.ApplyDefaults(),
	}

	service := NewQueryFrontendService(opts)

	if service.GetName() != opts.Name {
		t.Errorf("expected service name to be %s, got %s", opts.Name, service.GetName())
	}

	if service.GetNamespace() != opts.Namespace {
		t.Errorf("expected service namespace to be %s, got %s", opts.Namespace, service.GetNamespace())
	}

	if len(service.Spec.Ports) != 1 || service.Spec.Ports[0].Port != HTTPPort {
		t.Errorf("expected service to have 1 port (%d), got %v", HTTPPort, service.Spec.Ports)
	}

	expectedSelector := labelsForQueryFrontend(opts)
	if !equality.Semantic.DeepEqual(service.Spec.Selector, expectedSelector) {
		t.Errorf("expected service selector to be %v, got %v", expectedSelector, service.Spec.Selector)
	}
}

func TestNewQueryFrontendConfigMap(t *testing.T) {
	opts := QueryFrontendOptions{
		Options: manifests.Options{
			Name:      "test",
			Namespace: "ns",
			Labels: map[string]string{
				"some-custom-label":      someCustomLabelValue,
				"some-other-label":       someOtherLabelValue,
				"app.kubernetes.io/name": "expect-to-be-discarded",
			},
		}.ApplyDefaults(),
	}

	configMap := NewQueryFrontendInMemoryConfigMap(opts)

	// Cast configMap to *corev1.ConfigMap
	cm, ok := configMap.(*corev1.ConfigMap)
	if !ok {
		t.Fatalf("expected configMap to be *corev1.ConfigMap, got %T", configMap)
	}

	if cm.Data[defaultInMemoryConfigmapKey] != InMemoryConfig {
		t.Errorf("expected config map data to be %s, got %s", InMemoryConfig, cm.Data[defaultInMemoryConfigmapKey])
	}
}
