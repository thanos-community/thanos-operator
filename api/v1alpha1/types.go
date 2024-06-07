package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
)

// CommonThanosFields are the options available to all Thanos components.
// +k8s:deepcopy-gen=true
type CommonThanosFields struct {
	// Version of Thanos to be deployed.
	// If not specified, the operator assumes the latest upstream version of
	// Thanos available at the time when the version of the operator was
	// released.
	Version string `json:"version,omitempty"`
	// Container image to use for the Thanos components.
	// +optional
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

	// When a Thanos deployment is paused, no actions except for deletion
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
