package controller

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
)

func TestCompactVerticalCompaction(t *testing.T) {
	for _, tc := range []struct {
		name   string
		config *v1alpha1.CompactConfig
		args   []string
	}{
		{name: "without compact config"},
		{
			name: "with compact config",
			config: &v1alpha1.CompactConfig{
				CompactConcurrency:    ptr.To(int32(2)),
				BlockFetchConcurrency: ptr.To(int32(3)),
				CleanupInterval:       ptr.To(v1alpha1.Duration("3m")),
				ConsistencyDelay:      ptr.To(v1alpha1.Duration("15m")),
			},
			args: []string{
				"--compact.concurrency=2", "--compact.blocks-fetch-concurrency=3",
				"--compact.cleanup-interval=3m", "--consistency-delay=15m",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			compact := v1alpha1.ThanosCompact{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "monitoring"}}
			compact.Spec.CompactConfig = tc.config
			var existing *appsv1.StatefulSet
			for _, stage := range []struct {
				name   string
				config *v1alpha1.VerticalCompactionConfig
				args   []string
			}{
				{name: "unset"},
				{
					name: "enable",
					config: &v1alpha1.VerticalCompactionConfig{
						ReplicaLabels:     []string{"replica", "prometheus_replica"},
						DeduplicationFunc: ptr.To("penalty"),
					},
					args: []string{
						"--compact.enable-vertical-compaction", "--deduplication.replica-label=replica",
						"--deduplication.replica-label=prometheus_replica", "--deduplication.func=penalty",
					},
				},
				{
					name:   "replace",
					config: &v1alpha1.VerticalCompactionConfig{ReplicaLabels: []string{"new_replica"}},
					args:   []string{"--compact.enable-vertical-compaction", "--deduplication.replica-label=new_replica"},
				},
				{name: "remove"},
			} {
				t.Run(stage.name, func(t *testing.T) {
					compact.Spec.VerticalCompactionConfig = stage.config
					opts := compactV1Alpha1ToOptions(compactV1Alpha1TransformInput{CRD: compact})
					var desired *appsv1.StatefulSet
					for _, obj := range opts.Build() {
						if workload, ok := obj.(*appsv1.StatefulSet); ok {
							desired = workload
						}
					}
					require.NotNil(t, desired)
					if existing == nil {
						existing = desired.DeepCopy()
						existing.SetCreationTimestamp(metav1.Now())
					} else {
						require.NoError(t, manifests.MutateFuncFor(existing, desired)())
					}
					var compactionArgs []string
					for _, arg := range existing.Spec.Template.Spec.Containers[0].Args {
						if strings.HasPrefix(arg, "--compact.") || strings.HasPrefix(arg, "--deduplication.") || strings.HasPrefix(arg, "--consistency-delay=") {
							compactionArgs = append(compactionArgs, arg)
						}
					}
					want := append(append([]string{}, tc.args...), stage.args...)
					require.ElementsMatch(t, want, compactionArgs)
				})
			}
		})
	}
}

func TestImagePullSecrets(t *testing.T) {
	meta := metav1.ObjectMeta{Name: "test", Namespace: "monitoring"}
	query := v1alpha1.ThanosQuery{ObjectMeta: meta}
	query.Spec.QueryFrontend = &v1alpha1.QueryFrontendSpec{}
	receive := v1alpha1.ThanosReceive{ObjectMeta: meta}
	receive.Spec.Ingester.Hashrings = []v1alpha1.IngesterHashringSpec{{Name: "first"}, {Name: "second"}}
	ruler := v1alpha1.ThanosRuler{ObjectMeta: meta}
	store := v1alpha1.ThanosStore{ObjectMeta: meta}
	compact := v1alpha1.ThanosCompact{ObjectMeta: meta}

	tests := []struct {
		name   string
		common *v1alpha1.CommonFields
		build  func() manifests.Buildable
	}{
		{
			name:   "query",
			common: &query.Spec.CommonFields,
			build: func() manifests.Buildable {
				return queryV1Alpha1ToOptions(queryV1Alpha1TransformInput{CRD: query})
			},
		},
		{
			name:   "query-frontend",
			common: &query.Spec.QueryFrontend.CommonFields,
			build: func() manifests.Buildable {
				return queryV1Alpha1ToQueryFrontEndOptions(queryV1Alpha1ToQueryFrontEndTransformInput{CRD: query})
			},
		},
		{
			name:   "receive-router",
			common: &receive.Spec.Router.CommonFields,
			build: func() manifests.Buildable {
				return receiverV1Alpha1ToRouterOptions(receiverV1Alpha1ToRouterTransformInput{CRD: receive})
			},
		},
		{
			name:   "receive-ingester-first",
			common: &receive.Spec.Ingester.Hashrings[0].CommonFields,
			build: func() manifests.Buildable {
				return receiverV1Alpha1ToIngesterOptions(receiverV1Alpha1ToIngesterTransformInput{CRD: receive, Spec: receive.Spec.Ingester.Hashrings[0]})
			},
		},
		{
			name:   "receive-ingester-second",
			common: &receive.Spec.Ingester.Hashrings[1].CommonFields,
			build: func() manifests.Buildable {
				return receiverV1Alpha1ToIngesterOptions(receiverV1Alpha1ToIngesterTransformInput{CRD: receive, Spec: receive.Spec.Ingester.Hashrings[1]})
			},
		},
		{
			name:   "ruler",
			common: &ruler.Spec.CommonFields,
			build: func() manifests.Buildable {
				return rulerV1Alpha1ToOptions(rulerV1Alpha1TransformInput{CRD: ruler})
			},
		},
		{
			name:   "store",
			common: &store.Spec.CommonFields,
			build: func() manifests.Buildable {
				return storeV1Alpha1ToOptions(storeV1Alpha1TransformInput{CRD: store})
			},
		},
		{
			name:   "compact",
			common: &compact.Spec.CommonFields,
			build: func() manifests.Buildable {
				return compactV1Alpha1ToOptions(compactV1Alpha1TransformInput{CRD: compact})
			},
		},
	}

	for _, tt := range tests {
		tt.common.ImagePullSecrets = []corev1.LocalObjectReference{{Name: tt.name + "-registry"}}
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var existing client.Object
			for _, stage := range []struct {
				name    string
				secrets []corev1.LocalObjectReference
			}{
				{name: "create", secrets: tt.common.ImagePullSecrets},
				{name: "replace", secrets: []corev1.LocalObjectReference{{Name: "new-registry"}, {Name: "backup-registry"}}},
				{name: "remove"},
				{name: "empty", secrets: []corev1.LocalObjectReference{}},
			} {
				t.Run(stage.name, func(t *testing.T) {
					tt.common.ImagePullSecrets = stage.secrets
					workloads := 0
					for _, desired := range tt.build().Build() {
						var podSpec *corev1.PodSpec
						switch obj := desired.(type) {
						case *appsv1.Deployment:
							podSpec = &obj.Spec.Template.Spec
						case *appsv1.StatefulSet:
							podSpec = &obj.Spec.Template.Spec
						default:
							continue
						}
						workloads++
						require.Equal(t, stage.secrets, podSpec.ImagePullSecrets)
						if existing == nil {
							existing = desired.DeepCopyObject().(client.Object)
							existing.SetCreationTimestamp(metav1.Now())
						}
						require.NoError(t, manifests.MutateFuncFor(existing, desired)())
						switch obj := existing.(type) {
						case *appsv1.Deployment:
							require.Equal(t, stage.secrets, obj.Spec.Template.Spec.ImagePullSecrets)
						case *appsv1.StatefulSet:
							require.Equal(t, stage.secrets, obj.Spec.Template.Spec.ImagePullSecrets)
						}
					}
					require.Equal(t, 1, workloads)
				})
			}
		})
	}
}

func TestAdditionalToOpts(t *testing.T) {
	tests := []struct {
		name     string
		input    v1alpha1.Additional
		expected manifests.Additional
	}{
		{
			name:     "empty additional",
			input:    v1alpha1.Additional{},
			expected: manifests.Additional{},
		},
		{
			name: "all fields populated",
			input: v1alpha1.Additional{
				Args: []string{"--test-arg=value"},
				Containers: []corev1.Container{
					{Name: "sidecar", Image: "sidecar:latest"},
				},
				Volumes: []corev1.Volume{
					{Name: "test-volume"},
				},
				VolumeMounts: []corev1.VolumeMount{
					{Name: "test-mount", MountPath: "/test"},
				},
				Ports: []corev1.ContainerPort{
					{Name: "metrics", ContainerPort: 8080},
				},
				Env: []corev1.EnvVar{
					{Name: "TEST_ENV", Value: "test"},
				},
				ServicePorts: []corev1.ServicePort{
					{Name: "http", Port: 8080},
				},
				ConfigMaps: []string{"my-configmap", "another-configmap"},
				Secrets:    []string{"my-secret", "another-secret"},
			},
			expected: manifests.Additional{
				Args: []string{"--test-arg=value"},
				Containers: []corev1.Container{
					{Name: "sidecar", Image: "sidecar:latest"},
				},
				Volumes: []corev1.Volume{
					{Name: "test-volume"},
				},
				VolumeMounts: []corev1.VolumeMount{
					{Name: "test-mount", MountPath: "/test"},
				},
				Ports: []corev1.ContainerPort{
					{Name: "metrics", ContainerPort: 8080},
				},
				Env: []corev1.EnvVar{
					{Name: "TEST_ENV", Value: "test"},
				},
				ServicePorts: []corev1.ServicePort{
					{Name: "http", Port: 8080},
				},
				ConfigMaps: []string{"my-configmap", "another-configmap"},
				Secrets:    []string{"my-secret", "another-secret"},
			},
		},
		{
			name: "only configmaps and secrets",
			input: v1alpha1.Additional{
				ConfigMaps: []string{"config1", "config2"},
				Secrets:    []string{"secret1"},
			},
			expected: manifests.Additional{
				ConfigMaps: []string{"config1", "config2"},
				Secrets:    []string{"secret1"},
			},
		},
		{
			name: "only args and containers",
			input: v1alpha1.Additional{
				Args: []string{"--arg1", "--arg2"},
				Containers: []corev1.Container{
					{Name: "test"},
				},
			},
			expected: manifests.Additional{
				Args: []string{"--arg1", "--arg2"},
				Containers: []corev1.Container{
					{Name: "test"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := additionalToOpts(tt.input)

			if diff := cmp.Diff(tt.expected, result); diff != "" {
				t.Errorf("additionalToOpts() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
