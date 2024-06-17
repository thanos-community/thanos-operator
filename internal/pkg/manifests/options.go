package manifests

import (
	"fmt"

	"k8s.io/utils/ptr"
)

const (
	DefaultThanosImage   = "quay.io/thanos/thanos"
	DefaultThanosVersion = "v0.35.1"
)

// Options is a struct that holds the options for the common manifests
type Options struct {
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
	Version        *string
	containerImage string
}

// ApplyDefaults applies the default values to the options
func (o Options) ApplyDefaults() Options {
	if o.Image == nil || *o.Image == "" {
		o.Image = ptr.To(DefaultThanosImage)
	}

	if o.Version == nil || *o.Version == "" {
		o.Version = ptr.To(DefaultThanosVersion)
	}
	o.containerImage = fmt.Sprintf("%s:%s", *o.Image, *o.Version)

	return o
}

// GetContainerImage for the Options
func (o Options) GetContainerImage() string {
	return o.containerImage
}
