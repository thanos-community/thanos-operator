package manifests

import (
	corev1 "k8s.io/api/core/v1"
)

type Additional struct {
	// Additional arguments to pass to the Thanos components.
	Args []string `json:"additionalArgs,omitempty"`
	// Additional containers to add to the Thanos components.
	Containers []corev1.Container `json:"additionalContainers,omitempty"`
	// Additional volumes to add to the Thanos components.
	Volumes []corev1.Volume `json:"additionalVolumes,omitempty"`
	// Additional volume mounts to add to the Thanos component container in a Deployment or StatefulSet
	// controlled by the operator.
	VolumeMounts []corev1.VolumeMount `json:"additionalVolumeMounts,omitempty"`
	// Additional ports to expose on the Thanos component container in a Deployment or StatefulSet
	// controlled by the operator.
	Ports []corev1.ContainerPort `json:"additionalPorts,omitempty"`
	// Additional environment variables to add to the Thanos component container in a Deployment or StatefulSet
	// controlled by the operator.
	Env []corev1.EnvVar `json:"additionalEnv,omitempty"`
	// AdditionalServicePorts are additional ports to expose on the Service for the Thanos component.
	ServicePorts []corev1.ServicePort `json:"additionalServicePorts,omitempty"`
}
