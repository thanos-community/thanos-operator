package manifests

import (
	corev1 "k8s.io/api/core/v1"
)

type Additional struct {
	// Additional arguments to pass to the Thanos components.
	Args []string
	// Additional containers to add to the Thanos components.
	Containers []corev1.Container
	// Additional volumes to add to the Thanos components.
	Volumes []corev1.Volume
	// Additional volume mounts to add to the Thanos component container in a Deployment or StatefulSet
	// controlled by the operator.
	VolumeMounts []corev1.VolumeMount
	// Additional ports to expose on the Thanos component container in a Deployment or StatefulSet
	// controlled by the operator.
	Ports []corev1.ContainerPort
	// Additional environment variables to add to the Thanos component container in a Deployment or StatefulSet
	// controlled by the operator.
	Env []corev1.EnvVar
	// AdditionalServicePorts are additional ports to expose on the Service for the Thanos component.
	ServicePorts []corev1.ServicePort
}

type Duration string
