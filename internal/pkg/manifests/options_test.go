package manifests

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestOptions_GetContainerImage(t *testing.T) {
	tests := []struct {
		name string
		o    Options
		want string
	}{
		{
			name: "get default image",
			o:    Options{},
			want: DefaultThanosImage + ":" + DefaultThanosVersion,
		},
		{
			name: "get custom image from options",
			o: Options{
				Image:   ptr.To("quay.io/thanos/thanos"),
				Version: ptr.To("latest"),
			},
			want: "quay.io/thanos/thanos:latest",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.o.GetContainerImage(); got != tt.want {
				t.Errorf("Options.GetContainerImage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOptions_ToFlags(t *testing.T) {
	tests := []struct {
		name string
		o    Options
		want []string
	}{
		{
			name: "get default flags",
			o:    Options{},
			want: []string{
				fmt.Sprintf("--log.level=%s", defaultLogLevel),
				fmt.Sprintf("--log.format=%s", defaultLogFormat),
			},
		},
		{
			name: "get custom flags",
			o: Options{
				LogLevel:  ptr.To("debug"),
				LogFormat: ptr.To("json"),
			},
			want: []string{
				"--log.level=debug",
				"--log.format=json",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(tt.o.ToFlags(), tt.want) {
				t.Errorf("Options.ToFlags() = %v, want %v", tt.o.ToFlags(), tt.want)
			}
		})
	}
}

func TestRelabelConfig_String(t *testing.T) {
	tests := []struct {
		name string
		r    RelabelConfig
		want string
	}{
		{
			name: "get value for hashmod",
			r: RelabelConfig{
				SourceLabel: "any",
				TargetLabel: "some_target",
				Modulus:     1,
				Action:      "hashmod",
			},
			want: `
- action: hashmod
  source_labels: ["any"]
  target_label: some_target
  modulus: 1`,
		},
		{
			name: "get value for keep",
			r: RelabelConfig{
				SourceLabel: "any",
				TargetLabel: "some_target",
				Regex:       "^test",
				Action:      "keep",
			},
			want: `
- action: keep
  source_labels: ["any"]
  target_label: some_target
  regex: ^test`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.String(); got != tt.want {
				t.Errorf("RelabelConfig.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRelabelConfigs_ToFlags(t *testing.T) {
	tests := []struct {
		name string
		rc   RelabelConfigs
		want string
	}{
		{
			name: "get flags for relabel configs",
			rc: RelabelConfigs{
				{
					SourceLabel: "any",
					TargetLabel: "some_target",
					Modulus:     1,
					Action:      "hashmod",
				},
				{
					SourceLabel: "any",
					TargetLabel: "some_target",
					Regex:       "^test",
					Action:      "keep",
				},
			},
			want: "--selector.relabel-config=" + `
- action: hashmod
  source_labels: ["any"]
  target_label: some_target
  modulus: 1
- action: keep
  source_labels: ["any"]
  target_label: some_target
  regex: ^test`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rc.ToFlags(); got != tt.want {
				t.Errorf("RelabelConfigs.ToFlags() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInMemoryCacheConfig_String(t *testing.T) {
	tests := []struct {
		name     string
		config   InMemoryCacheConfig
		expected string
	}{
		{
			name:   "EmptyConfig",
			config: InMemoryCacheConfig{},
			expected: `type: IN-MEMORY
config:
`,
		},
		{
			name:   "WithMaxSize",
			config: InMemoryCacheConfig{MaxSize: "100MB"},
			expected: `type: IN-MEMORY
config:
  max_size: 100MB
`,
		},
		{
			name:   "WithMaxItemSize",
			config: InMemoryCacheConfig{MaxItemSize: "10MB"},
			expected: `type: IN-MEMORY
config:
  max_item_size: 10MB
`,
		},
		{
			name:   "WithMaxSizeAndMaxItemSize",
			config: InMemoryCacheConfig{MaxSize: "100MB", MaxItemSize: "10MB"},
			expected: `type: IN-MEMORY
config:
  max_size: 100MB
  max_item_size: 10MB
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.String(); got != tt.expected {
				t.Errorf("InMemoryCacheConfig.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

type mockOptionsForGolden struct {
	Options
}

const mockImage = "quay.io/thanos/thanos:mock-latest"

func (m mockOptionsForGolden) GetContainerImage() string {
	return mockImage
}

func TestAugmentWithOptions_Deployment_Golden(t *testing.T) {
	tests := []struct {
		name    string
		objInit func() *appsv1.Deployment
		opts    mockOptionsForGolden
	}{
		{
			name: "deployment-basic",
			objInit: func() *appsv1.Deployment {
				return &appsv1.Deployment{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Deployment",
						APIVersion: "apps/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "thanos-query",
						Namespace: "monitoring",
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "thanos-query",
							},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "thanos-query",
								},
							},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: "thanos",
										Args: []string{"query", "--log.level=info"},
									},
								},
							},
						},
					},
				}
			},
			opts: mockOptionsForGolden{
				Options: Options{
					Owner:     "thanos-controller",
					Namespace: "monitoring",
					Replicas:  2,
					Labels: map[string]string{
						"app.kubernetes.io/name": "thanos-query",
					},
					Annotations: map[string]string{
						"prometheus.io/scrape": "true",
					},
					LogLevel: ptr.To("debug"),
					ResourceRequirements: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("512Mi"),
						},
					},
				},
			},
		},
		{
			name: "deployment-complete",
			objInit: func() *appsv1.Deployment {
				return &appsv1.Deployment{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Deployment",
						APIVersion: "apps/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "thanos-query",
						Namespace: "monitoring",
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "thanos-query",
							},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "thanos-query",
								},
							},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: "thanos",
										Args: []string{"query", "--log.level=info"},
									},
								},
							},
						},
					},
				}
			},
			opts: mockOptionsForGolden{

				Options: Options{
					Owner:     "thanos-controller",
					Namespace: "monitoring",
					Replicas:  2,
					Labels: map[string]string{
						"app.kubernetes.io/name": "thanos-query",
					},
					Annotations: map[string]string{
						"prometheus.io/scrape": "true",
					},
					LogLevel:  ptr.To("debug"),
					LogFormat: ptr.To("json"),
					ResourceRequirements: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("512Mi"),
						},
					},
					Additional: Additional{
						Args: []string{"--query.timeout=5m", "--query.max-concurrent=20"},
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
											Name: "thanos-query-config",
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
						Ports: []corev1.ContainerPort{
							{
								Name:          "http",
								ContainerPort: 10902,
								Protocol:      corev1.ProtocolTCP,
							},
							{
								Name:          "grpc",
								ContainerPort: 10901,
								Protocol:      corev1.ProtocolTCP,
							},
						},
						Env: []corev1.EnvVar{
							{
								Name:  "THANOS_LOG_FORMAT",
								Value: "json",
							},
						},
						ServicePorts: []corev1.ServicePort{
							{
								Name:     "http",
								Port:     10902,
								Protocol: corev1.ProtocolTCP,
							},
						},
					},
					PlacementConfig: &Placement{
						NodeSelector: map[string]string{
							"disktype": "ssd",
						},
						Tolerations: []corev1.Toleration{
							{
								Key:      "example-key",
								Operator: corev1.TolerationOpExists,
								Effect:   corev1.TaintEffectNoSchedule,
							},
						},
						Affinity: &corev1.Affinity{
							NodeAffinity: &corev1.NodeAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{
										{
											MatchExpressions: []corev1.NodeSelectorRequirement{
												{
													Key:      "kubernetes.io/e2e-az-name",
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{"e2e-az1", "e2e-az2"},
												},
											},
										},
									},
								},
							},
						},
					},
					ServiceMonitorConfig: &ServiceMonitorConfig{},
					PodDisruptionConfig:  &PodDisruptionBudgetOptions{},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := tt.objInit()
			AugmentWithOptions(obj, tt.opts.Options)

			// Create golden file path
			goldenFilePath := filepath.Join("testdata", tt.name+".golden.json")

			if *update {
				// Update golden file
				bytes, err := json.MarshalIndent(obj, "", "  ")
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

			assert.Equal(t, expected.String(), obj.String())
		})
	}
}

// Golden file test for StatefulSet
func TestAugmentWithOptions_StatefulSet_Golden(t *testing.T) {
	tests := []struct {
		name    string
		objInit func() *appsv1.StatefulSet
		opts    mockOptionsForGolden
	}{
		{
			name: "statefulset-basic",
			objInit: func() *appsv1.StatefulSet {
				return &appsv1.StatefulSet{
					TypeMeta: metav1.TypeMeta{
						Kind:       "StatefulSet",
						APIVersion: "apps/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "thanos-store",
						Namespace: "monitoring",
					},
					Spec: appsv1.StatefulSetSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "thanos-store",
							},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "thanos-store",
								},
							},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: "thanos",
										Args: []string{"store", "--log.level=info"},
									},
								},
							},
						},
					},
				}
			},
			opts: mockOptionsForGolden{
				Options: Options{
					Owner:     "thanos-controller",
					Namespace: "monitoring",
					Replicas:  3,
					Labels: map[string]string{
						"app.kubernetes.io/name": "thanos-store",
					},
					Annotations: map[string]string{
						"prometheus.io/scrape": "true",
					},
					LogLevel: ptr.To("debug"),
					ResourceRequirements: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("2"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				},
			},
		},
		{
			name: "statefulset-complete",
			objInit: func() *appsv1.StatefulSet {
				return &appsv1.StatefulSet{
					TypeMeta: metav1.TypeMeta{
						Kind:       "StatefulSet",
						APIVersion: "apps/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "thanos-store",
						Namespace: "monitoring",
					},
					Spec: appsv1.StatefulSetSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "thanos-store",
							},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "thanos-store",
								},
							},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: "thanos",
										Args: []string{"store", "--log.level=info"},
									},
								},
							},
						},
					},
				}
			},
			opts: mockOptionsForGolden{
				Options: Options{
					Owner:     "thanos-controller",
					Namespace: "monitoring",
					Replicas:  3,
					Labels: map[string]string{
						"app.kubernetes.io/name": "thanos-store",
					},
					Annotations: map[string]string{
						"prometheus.io/scrape": "true",
					},
					LogLevel:  ptr.To("debug"),
					LogFormat: ptr.To("json"),
					ResourceRequirements: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("2"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
					Additional: Additional{
						Args: []string{"--store.grpc-series-max-concurrency=20"},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "data",
								MountPath: "/data",
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "config",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "thanos-store-config",
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
						Ports: []corev1.ContainerPort{
							{
								Name:          "http",
								ContainerPort: 10902,
								Protocol:      corev1.ProtocolTCP,
							},
							{
								Name:          "grpc",
								ContainerPort: 10901,
								Protocol:      corev1.ProtocolTCP,
							},
						},
						Env: []corev1.EnvVar{
							{
								Name:  "THANOS_LOG_FORMAT",
								Value: "json",
							},
						},
						ServicePorts: []corev1.ServicePort{
							{
								Name:     "http",
								Port:     10902,
								Protocol: corev1.ProtocolTCP,
							},
						},
					},
					PlacementConfig: &Placement{
						NodeSelector: map[string]string{
							"disktype": "ssd",
						},
						Tolerations: []corev1.Toleration{
							{
								Key:      "example-key",
								Operator: corev1.TolerationOpExists,
								Effect:   corev1.TaintEffectNoSchedule,
							},
						},
						Affinity: &corev1.Affinity{
							NodeAffinity: &corev1.NodeAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{
										{
											MatchExpressions: []corev1.NodeSelectorRequirement{
												{
													Key:      "kubernetes.io/e2e-az-name",
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{"e2e-az1", "e2e-az2"},
												},
											},
										},
									},
								},
							},
						},
					},
					ServiceMonitorConfig: &ServiceMonitorConfig{},
					PodDisruptionConfig:  &PodDisruptionBudgetOptions{},
					StatefulSet: StatefulSet{
						PodManagementPolicy: "Parallel",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := tt.objInit()
			AugmentWithOptions(obj, tt.opts.Options)

			// Create golden file path
			goldenFilePath := filepath.Join("testdata", tt.name+".golden.json")

			if *update {
				// Update golden file
				bytes, err := json.MarshalIndent(obj, "", "  ")
				require.NoError(t, err)

				err = os.MkdirAll(filepath.Dir(goldenFilePath), 0755)
				require.NoError(t, err)

				err = os.WriteFile(goldenFilePath, bytes, 0644)
				require.NoError(t, err)
			}

			// Read golden file
			bytes, err := os.ReadFile(goldenFilePath)
			require.NoError(t, err)

			var expected appsv1.StatefulSet
			err = json.Unmarshal(bytes, &expected)
			require.NoError(t, err)

			assert.Equal(t, expected.String(), obj.String())
		})
	}
}

var update = flag.Bool("update", false, "update golden files")

func TestAugmentWithOptions_UnsupportedType(t *testing.T) {
	// Test that passing an unsupported type doesn't modify the object or cause errors
	obj := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
	}

	originalObj := obj.DeepCopy()

	opts := mockOptionsForGolden{
		Options: Options{
			Owner:     "controller",
			Namespace: "default",
		},
	}

	// This should be a no-op for Service
	AugmentWithOptions(obj, opts.Options)
	// Verify the object wasn't modified
	assert.Equal(t, originalObj, obj)
}
