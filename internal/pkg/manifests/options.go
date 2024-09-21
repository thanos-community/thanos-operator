package manifests

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DefaultThanosImage   = "quay.io/thanos/thanos"
	DefaultThanosVersion = "v0.35.1"

	defaultLogLevel  = "info"
	defaultLogFormat = "logfmt"
)

// Options is a struct that holds the options for the common manifests
type Options struct {
	Additional
	// Name is the name of the object
	Name string
	// Namespace is the namespace of the object
	Namespace string
	// Replicas is the number of replicas for the object
	Replicas int32
	// Labels is the labels for the object
	Labels map[string]string
	// Image is the image to use for the component
	Image *string
	// Version is the version of Thanos
	Version *string
	// ResourceRequirements for the component
	ResourceRequirements *corev1.ResourceRequirements
	// LogLevel is the log level for the component
	LogLevel *string
	// LogFormat is the log format for the component
	LogFormat *string
}

// ToFlags returns the flags for the Options
func (o Options) ToFlags() []string {
	if o.LogLevel == nil || *o.LogLevel == "" {
		o.LogLevel = ptr.To(defaultLogLevel)
	}

	if o.LogFormat == nil || *o.LogFormat == "" {
		o.LogFormat = ptr.To(defaultLogFormat)
	}

	return []string{
		fmt.Sprintf("--log.level=%s", *o.LogLevel),
		fmt.Sprintf("--log.format=%s", *o.LogFormat),
	}
}

// GetContainerImage for the Options
func (o Options) GetContainerImage() string {
	if o.Image == nil || *o.Image == "" {
		o.Image = ptr.To(DefaultThanosImage)
	}

	if o.Version == nil || *o.Version == "" {
		o.Version = ptr.To(DefaultThanosVersion)
	}
	return fmt.Sprintf("%s:%s", *o.Image, *o.Version)
}

// AugmentWithOptions augments the object with the options
// Supported objects are Deployment and StatefulSet and ServiceAccount
func AugmentWithOptions(obj client.Object, opts Options) {
	switch o := obj.(type) {
	case *appsv1.Deployment:
		o.Spec.Template.Spec.Containers[0].Image = opts.GetContainerImage()

		if opts.ResourceRequirements != nil {
			o.Spec.Template.Spec.Containers[0].Resources = *opts.ResourceRequirements
		}

		if opts.Additional.VolumeMounts != nil {
			o.Spec.Template.Spec.Containers[0].VolumeMounts = append(
				o.Spec.Template.Spec.Containers[0].VolumeMounts,
				opts.Additional.VolumeMounts...)
		}

		if opts.Additional.Containers != nil {
			o.Spec.Template.Spec.Containers = append(
				o.Spec.Template.Spec.Containers,
				opts.Additional.Containers...)
		}

		if opts.Additional.Volumes != nil {
			o.Spec.Template.Spec.Volumes = append(
				o.Spec.Template.Spec.Volumes,
				opts.Additional.Volumes...)
		}

		if opts.Additional.Ports != nil {
			o.Spec.Template.Spec.Containers[0].Ports = append(
				o.Spec.Template.Spec.Containers[0].Ports,
				opts.Additional.Ports...)
		}

		if opts.Additional.Env != nil {
			o.Spec.Template.Spec.Containers[0].Env = append(
				o.Spec.Template.Spec.Containers[0].Env,
				opts.Additional.Env...)
		}
	case *appsv1.StatefulSet:
		o.Spec.Template.Spec.Containers[0].Image = opts.GetContainerImage()

		if opts.ResourceRequirements != nil {
			o.Spec.Template.Spec.Containers[0].Resources = *opts.ResourceRequirements
		}

		if opts.Additional.VolumeMounts != nil {
			o.Spec.Template.Spec.Containers[0].VolumeMounts = append(
				o.Spec.Template.Spec.Containers[0].VolumeMounts,
				opts.Additional.VolumeMounts...)
		}

		if opts.Additional.Containers != nil {
			o.Spec.Template.Spec.Containers = append(
				o.Spec.Template.Spec.Containers,
				opts.Additional.Containers...)
		}

		if opts.Additional.Volumes != nil {
			o.Spec.Template.Spec.Volumes = append(
				o.Spec.Template.Spec.Volumes,
				opts.Additional.Volumes...)
		}

		if opts.Additional.Ports != nil {
			o.Spec.Template.Spec.Containers[0].Ports = append(
				o.Spec.Template.Spec.Containers[0].Ports,
				opts.Additional.Ports...)
		}

		if opts.Additional.Env != nil {
			o.Spec.Template.Spec.Containers[0].Env = append(
				o.Spec.Template.Spec.Containers[0].Env,
				opts.Additional.Env...)
		}
	default:
		//no-op
	}
}
