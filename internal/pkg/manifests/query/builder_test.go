package query

import (
	"reflect"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/test/utils"

	"gotest.tools/v3/golden"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	someCustomLabelValue string = "xyz"
	someOtherLabelValue  string = "abc"
)

func TestBuildQuery(t *testing.T) {
	opts := Options{
		Options: manifests.Options{
			Owner:     "any",
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
			PodDisruptionConfig: &manifests.PodDisruptionBudgetOptions{},
		},
		Timeout:       "15m",
		LookbackDelta: "5m",
		MaxConcurrent: 20,
	}

	objs := opts.Build()
	if len(objs) != 5 {
		t.Fatalf("expected 5 objects, got %d", len(objs))
	}

	for _, obj := range objs {
		assert.Assert(t, obj.GetAnnotations()["test"] == "annotation")
	}

	utils.ValidateIsNamedServiceAccount(t, objs[0], opts, opts.Namespace)
	cm, ok := objs[1].(*corev1.ConfigMap)
	assert.Assert(t, ok)
	assert.Equal(t, opts.GetGeneratedResourceName(), cm.GetName())
	assert.Assert(t, cm.Data[endpointSDConfigKey] != "")
	utils.ValidateObjectsEqual(t, objs[2], NewQueryDeployment(opts))
	utils.ValidateObjectsEqual(t, objs[3], NewQueryService(opts))
	utils.ValidateIsNamedPodDisruptionBudget(t, objs[4], opts, opts.Namespace, objs[2])

	wantLabels := opts.GetSelectorLabels()
	wantLabels["some-custom-label"] = someCustomLabelValue
	wantLabels["some-other-label"] = someOtherLabelValue
	utils.ValidateObjectLabelsEqual(t, wantLabels, []client.Object{objs[1], objs[2], objs[3]}...)
}

func TestNewQueryDeployment(t *testing.T) {

	for _, tc := range []struct {
		name   string
		golden string
		opts   Options
	}{
		{
			name:   "test query deployment correctness",
			golden: "deployment-basic.golden.yaml",
			opts: Options{
				Options: manifests.Options{
					Owner:     "test-q",
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
					Annotations: map[string]string{
						"test":    "annotation",
						"another": "annotation",
					},
				},
				Timeout:       "15m",
				LookbackDelta: "5m",
				MaxConcurrent: 20,
			},
		},
		{
			name:   "test additional volumemount",
			golden: "deployment-with-volumemount.golden.yaml",
			opts: Options{
				Options: manifests.Options{
					Owner:     "test-q",
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
					Annotations: map[string]string{
						"test":    "annotation",
						"another": "annotation",
					},
					Additional: manifests.Additional{
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "test-sd",
								MountPath: "/test-sd-file",
							},
						},
					},
				},
				Timeout:       "15m",
				LookbackDelta: "5m",
				MaxConcurrent: 20,
			},
		},
		{
			name:   "test additional container",
			golden: "deployment-with-container.golden.yaml",
			opts: Options{
				Options: manifests.Options{
					Owner:     "test-q",
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
					Annotations: map[string]string{
						"test":    "annotation",
						"another": "annotation",
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
				Timeout:       "15m",
				LookbackDelta: "5m",
				MaxConcurrent: 20,
			},
		},
		{
			name:   "test with otel sidecar enabled",
			golden: "deployment-with-otel-sidecar.golden.yaml",
			opts: Options{
				Options: manifests.Options{
					Owner:     "test-q",
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
					Config: featuregate.Config{
						OtelSidecar: featuregate.Enabled(),
					},
				},
				Timeout:       "15m",
				LookbackDelta: "5m",
				MaxConcurrent: 20,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			query := NewQueryDeployment(tc.opts)

			// Test against golden file
			yamlBytes, err := yaml.Marshal(query)
			if err != nil {
				t.Fatalf("failed to marshal deployment to YAML: %v", err)
			}
			golden.Assert(t, string(yamlBytes), tc.golden)
		})
	}
}

func TestNewQueryService(t *testing.T) {
	opts := Options{
		Options: manifests.Options{
			Owner:     "test-q",
			Namespace: "ns",
			Image:     ptr.To("some-custom-image"),
			Labels: map[string]string{
				"some-custom-label":      someCustomLabelValue,
				"some-other-label":       someOtherLabelValue,
				"app.kubernetes.io/name": "expect-to-be-discarded",
			},
			Annotations: map[string]string{
				"test":    "annotation",
				"another": "annotation",
			},
		},
		Timeout:       "15m",
		LookbackDelta: "5m",
		MaxConcurrent: 20,
	}

	for _, tc := range []struct {
		name   string
		golden string
		opts   Options
	}{
		{
			name:   "test query service correctness",
			golden: "service-basic.golden.yaml",
			opts:   opts,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			querySvc := NewQueryService(tc.opts)

			// Test against golden file
			yamlBytes, err := yaml.Marshal(querySvc)
			if err != nil {
				t.Fatalf("failed to marshal service to YAML: %v", err)
			}
			golden.Assert(t, string(yamlBytes), tc.golden)
		})
	}
}

// TestBuildQueryGolden uses golden files to validate complete manifest generation
func TestBuildQueryGolden(t *testing.T) {
	tests := []struct {
		name string
		opts Options
	}{
		{
			name: "query-basic",
			opts: Options{
				Options: manifests.Options{
					Owner:     "test-owner",
					Namespace: "test-namespace",
					Image:     ptr.To("quay.io/thanos/thanos:v0.40.1"),
					Labels: map[string]string{
						"app.kubernetes.io/version": "v0.40.1",
					},
					Annotations: map[string]string{
						"test":    "annotation",
						"another": "annotation",
					},
					PodDisruptionConfig: &manifests.PodDisruptionBudgetOptions{},
				},
				Timeout:       "15m",
				LookbackDelta: "5m",
				MaxConcurrent: 20,
			},
		},
		{
			name: "query-complete",
			opts: Options{
				Options: manifests.Options{
					Owner:     "test-owner",
					Namespace: "test-namespace",
					Image:     ptr.To("quay.io/thanos/thanos:v0.40.1"),
					Labels: map[string]string{
						"app.kubernetes.io/version": "v0.40.1",
						"custom-label":              "custom-value",
					},
					Annotations: map[string]string{
						"test-annotation": "test-value",
						"another":         "annotation",
					},
					Additional: manifests.Additional{
						Args: []string{"--query.timeout=30m"},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "config",
								MountPath: "/etc/thanos",
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "config",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "query-config",
										},
									},
								},
							},
						},
						Containers: []corev1.Container{
							{
								Name:  "sidecar",
								Image: "sidecar:latest",
								Args:  []string{"--verbose"},
							},
						},
					},
					PodDisruptionConfig: &manifests.PodDisruptionBudgetOptions{},
				},
				Timeout:       "30m",
				LookbackDelta: "10m",
				MaxConcurrent: 50,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs := tt.opts.Build()

			// Validate complete manifest set
			yamlBytes, err := yaml.Marshal(objs)
			if err != nil {
				t.Fatalf("failed to marshal objects to YAML: %v", err)
			}
			golden.Assert(t, string(yamlBytes), tt.name+".golden.yaml")
		})
	}
}

func TestQueryArgsEndpoints(t *testing.T) {
	endpoints := []Endpoint{
		{ServiceName: "store", Namespace: "ns", Type: manifests.RegularLabel, Port: 10901},
		{ServiceName: "store-strict", Namespace: "ns", Type: manifests.StrictLabel, Port: 10901},
		{ServiceName: "store-group", Namespace: "ns", Type: manifests.GroupLabel, Port: 10901},
		{ServiceName: "store-group-strict", Namespace: "ns", Type: manifests.GroupStrictLabel, Port: 10901},
	}
	opts := Options{
		Options: manifests.Options{
			Owner:     "test-owner",
			Namespace: "ns",
		},
		Endpoints: endpoints,
	}

	args := queryArgs(opts)
	assert.Assert(t, contains(args, "--endpoint.sd-config-file=/etc/thanos/endpoint-sd/endpoints.yaml"), "got %v", args)
	assert.Assert(t, contains(args, "--endpoint.sd-config-reload-interval=30s"), "got %v", args)
	for _, arg := range args {
		for _, forbidden := range []string{
			"--endpoint=",
			"--endpoint-strict=",
			"--endpoint-group=",
			"--endpoint-group-strict=",
		} {
			assert.Assert(t, !strings.Contains(arg, forbidden), "query args contain %q in %q", forbidden, arg)
		}
	}

	var cm *corev1.ConfigMap
	for _, obj := range opts.Build() {
		if found, ok := obj.(*corev1.ConfigMap); ok {
			cm = found
			break
		}
	}
	assert.Assert(t, cm != nil, "Build() did not emit a ConfigMap")

	var got wireEndpointSDFile
	assert.NilError(t, yaml.Unmarshal([]byte(cm.Data["endpoints.yaml"]), &got))
	assert.DeepEqual(t, got.Endpoints, []wireEndpointSD{
		{Address: "dnssrv+_grpc._tcp.store.ns.svc"},
		{Address: "dnssrv+_grpc._tcp.store-group.ns.svc", Group: true},
		{Address: "store-group-strict.ns.svc:10901", Group: true, Strict: true},
		{Address: "store-strict.ns.svc:10901", Strict: true},
	})

	without := opts
	without.Endpoints = nil
	assert.Assert(t,
		reflect.DeepEqual(NewQueryDeployment(opts).Spec.Template, NewQueryDeployment(without).Spec.Template),
		"pod template changed when endpoints changed",
	)

	empty := renderEndpointSDConfig(nil)
	assert.Assert(t, strings.Contains(empty, "[]"), "empty document was %q", empty)
	assert.Assert(t, !strings.Contains(empty, "null"), "empty document was %q", empty)
	var emptyDoc wireEndpointSDFile
	assert.NilError(t, yaml.Unmarshal([]byte(empty), &emptyDoc))
	assert.Equal(t, 0, len(emptyDoc.Endpoints))
}

func TestQueryRejectsReservedEndpointVolume(t *testing.T) {
	opts := Options{Options: manifests.Options{Owner: "test-owner", Namespace: "ns"}}
	assert.NilError(t, opts.Valid())

	opts.Additional.Volumes = []corev1.Volume{{Name: endpointSDVolumeName}}
	assert.ErrorContains(t, opts.Valid(), endpointSDVolumeName)

	opts.Additional.Volumes = nil
	opts.Additional.VolumeMounts = []corev1.VolumeMount{{Name: "user", MountPath: endpointSDMountPath}}
	assert.ErrorContains(t, opts.Valid(), endpointSDMountPath)
}

type wireEndpointSDFile struct {
	Endpoints []wireEndpointSD `json:"endpoints"`
}

type wireEndpointSD struct {
	Address string `json:"address"`
	Group   bool   `json:"group"`
	Strict  bool   `json:"strict"`
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
