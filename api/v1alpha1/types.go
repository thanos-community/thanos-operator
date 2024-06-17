package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
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
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// An optional list of references to Secrets in the same namespace
	// to use for pulling images from registries.
	// See http://kubernetes.io/docs/user-guide/images#specifying-imagepullsecrets-on-a-pod
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// When a resource is paused, no actions except for deletion
	// will be performed on the underlying objects.
	Paused bool `json:"paused,omitempty"`

	// Log level for Thanos.
	// +kubebuilder:validation:Enum=debug;info;warn;error
	// +kubebuilder:default:=info
	LogLevel string `json:"logLevel,omitempty" opt:"log.level"`
	// Log format for Thanos.
	// +kubebuilder:validation:Enum=logfmt;json
	// +kubebuilder:default:=logfmt
	LogFormat string `json:"logFormat,omitempty" opt:"log.format"`
}
