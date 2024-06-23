package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

// Duration is a valid time duration that can be parsed by Prometheus model.ParseDuration() function.
// Supported units: y, w, d, h, m, s, ms
// Examples: `30s`, `1m`, `1h20m15s`, `15d`
// +kubebuilder:validation:Pattern:="^(0|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$"
type Duration string

// ObjectStorageConfig is the secret that contains the object storage configuration.
// The secret needs to be in the same namespace as the ReceiveHashring object.
// See https://thanos.io/tip/thanos/storage.md/#supported-clients for relevant documentation.
type ObjectStorageConfig corev1.SecretKeySelector

// ExternalLabels are the labels to add to the metrics.
// POD_NAME and POD_NAMESPACE are available via the downward API.
// +kubebuilder:validation:MinProperties=1
// +kubebuilder:validation:Required
// https://thanos.io/tip/thanos/storage.md/#external-labels
type ExternalLabels map[string]string

// CommonThanosFields are the options available to all Thanos components.
// +k8s:deepcopy-gen=true
type CommonThanosFields struct {
	// Version of Thanos to be deployed.
	// If not specified, the operator assumes the latest upstream version of
	// Thanos available at the time when the version of the operator was released.
	// +kubebuilder:validation:Optional
	Version *string `json:"version,omitempty"`
	// Container image to use for the Thanos components.
	// +kubebuilder:validation:Optional
	Image *string `json:"image,omitempty"`
	// Image pull policy for the Thanos containers.
	// See https://kubernetes.io/docs/concepts/containers/images/#image-pull-policy for more details.
	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	// +kubebuilder:default:=IfNotPresent
	// +kubebuilder:validation:Optional
	ImagePullPolicy *corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// An optional list of references to Secrets in the same namespace
	// to use for pulling images from registries.
	// See http://kubernetes.io/docs/user-guide/images#specifying-imagepullsecrets-on-a-pod
	// +kubebuilder:validation:Optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	// When a resource is paused, no actions except for deletion
	// will be performed on the underlying objects.
	// +kubebuilder:validation:Optional
	Paused *bool `json:"paused,omitempty"`
	// Log level for Thanos.
	// +kubebuilder:validation:Enum=debug;info;warn;error
	// +kubebuilder:validation:Optional
	LogLevel *string `json:"logLevel,omitempty"`
	// Log format for Thanos.
	// +kubebuilder:validation:Enum=logfmt;json
	// +kubebuilder:default:=logfmt
	// +kubebuilder:validation:Optional
	LogFormat *string `json:"logFormat,omitempty"`
	// Additional configuration for the Thanos components. Allows you to add
	// additional args, containers, volumes, and volume mounts to Thanos Deployments,
	// and StatefulSets. Ideal to use for things like sidecars.
	// +kubebuilder:validation:Optional
	Additional Additional `json:"additional,omitempty"`
}

type Additional struct {
	// Additional arguments to pass to the Thanos components.
	// +kubebuilder:validation:Optional
	AdditionalArgs []string `json:"additionalArgs,omitempty"`
	// Additional containers to add to the Thanos components.
	// +kubebuilder:validation:Optional
	AdditionalContainers []corev1.Container `json:"additionalContainers,omitempty"`
	// Additional volumes to add to the Thanos components.
	// +kubebuilder:validation:Optional
	AdditionalVolumes []corev1.Volume `json:"additionalVolumes,omitempty"`
	// Additional volume mounts to add to the Thanos component container in a Deployment or StatefulSet
	// controlled by the operator.
	// +kubebuilder:validation:Optional
	AdditionalVolumeMounts []corev1.VolumeMount `json:"additionalVolumeMounts,omitempty"`
	// Additional ports to expose on the Thanos component container in a Deployment or StatefulSet
	// controlled by the operator.
	// +kubebuilder:validation:Optional
	AdditionalPorts []corev1.ContainerPort `json:"additionalPorts,omitempty"`
	// Additional environment variables to add to the Thanos component container in a Deployment or StatefulSet
	// controlled by the operator.
	// +kubebuilder:validation:Optional
	AdditionalEnv []corev1.EnvVar `json:"additionalEnv,omitempty"`
	// AdditionalServicePorts are additional ports to expose on the Service for the Thanos component.
	// +kubebuilder:validation:Optional
	AdditionalServicePorts []corev1.ServicePort `json:"additionalServicePorts,omitempty"`
}

func (osc *ObjectStorageConfig) ToSecretKeySelector() corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: osc.Name},
		Key:                  osc.Key,
		Optional:             ptr.To(false),
	}
}
