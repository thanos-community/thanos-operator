package controller

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"

	"github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
)

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
