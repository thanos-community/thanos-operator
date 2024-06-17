/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RouterSpec represents the configuration for the router
type RouterSpec struct {
	// Labels are additional labels to add to the router components.
	// Labels set here will overwrite the labels inherited from the ThanosReceive object if they have the same key.
	// +kubebuilder:validation:Optional
	Labels map[string]string `json:"labels,omitempty"`
	// Replicas is the number of router replicas.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	// +kubebuilder:validation:Required
	Replicas int32 `json:"replicas,omitempty"`
	// ReplicationFactor is the replication factor for the router.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Enum=1;3;5
	// +kubebuilder:validation:Required
	ReplicationFactor int32 `json:"replicationFactor,omitempty"`
}

// IngesterSpec represents the configuration for the ingestor
type IngesterSpec struct {
	// DefaultObjectStorageConfig is the secret that contains the object storage configuration for the ingest components.
	// Can be overridden by the ObjectStorageConfig in the IngestorHashringSpec per hashring.
	// +kubebuilder:validation:Required
	DefaultObjectStorageConfig ObjectStorageConfig `json:"defaultObjectStorageConfig,omitempty"`
	// Hashrings is a list of hashrings to route to.
	// +kubebuilder:validation:MaxItems=100
	// +kubebuilder:validation:Required
	// +listType=map
	// +listMapKey=name
	Hashrings []IngestorHashringSpec `json:"hashrings,omitempty"`
}

// IngestorHashringSpec represents the configuration for a hashring to be used by the Thanos Receive StatefulSet.
type IngestorHashringSpec struct {
	// Name is the name of the hashring.
	// Name will be used to generate the names for the resources created for the hashring.
	// By default, Name will be used as a prefix with the ThanosReceive name as a suffix separated by a hyphen.
	// In cases where that name does not match the pattern below, i.e. the name is not a valid DNS-1123 subdomain,
	// the Name will be used as is and must be unique within the namespace.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^$|^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	Name string `json:"name"`
	// Labels are additional labels to add to the hashring components.
	// Labels set here will overwrite the labels inherited from the ThanosReceive object if they have the same key.
	// +kubebuilder:validation:Optional
	Labels map[string]string `json:"labels,omitempty"`
	// Replicas is the number of replicas/members of the hashring to add to the Thanos Receive StatefulSet.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	// +kubebuilder:validation:Required
	Replicas int32 `json:"replicas,omitempty"`
	// Retention is the duration for which the Thanos Receive StatefulSet will retain data.
	// +kubebuilder:default="2h"
	// +kubebuilder:validation:Optional
	Retention *Duration `json:"retention,omitempty"`
	// ObjectStorageConfig is the secret that contains the object storage configuration for the hashring.
	// +kubebuilder:validation:Optional
	ObjectStorageConfig *ObjectStorageConfig `json:"objectStorageConfig,omitempty"`
	// StorageSize is the size of the storage to be used by the Thanos Receive StatefulSet.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$`
	StorageSize string `json:"storageSize"`
	// Tenants is a list of tenants that should be matched by the hashring.
	// An empty list matches all tenants.
	// +kubebuilder:validation:Optional
	Tenants []string `json:"tenants,omitempty"`
	// TenantMatcherType is the type of tenant matching to use.
	// +kubebuilder:default:="exact"
	// +kubebuilder:validation:Enum=exact;glob
	TenantMatcherType string `json:"tenantMatcherType,omitempty"`
}

// ThanosReceiveSpec defines the desired state of ThanosReceive
// +kubebuilder:validation:XValidation:rule="self.ingestor.hashrings.all(h, h.replicas >= self.router.replicationFactor )", message=" Ingester replicas must be greater than or equal to the Router replicas"
type ThanosReceiveSpec struct {
	// CommonThanosFields are the options available to all Thanos components.
	CommonThanosFields `json:",inline"`
	// Router is the configuration for the router.
	// +kubebuilder:validation:Required
	Router RouterSpec `json:"router,omitempty"`
	// Ingester is the configuration for the ingestor.
	// +kubebuilder:validation:Required
	Ingester IngesterSpec `json:"ingestor,omitempty"`
}

// ThanosReceiveStatus defines the observed state of ThanosReceive
type ThanosReceiveStatus struct {
	// Conditions represent the latest available observations of the state of the hashring.
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ThanosReceive is the Schema for the thanosreceives API
type ThanosReceive struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of ThanosReceive
	Spec ThanosReceiveSpec `json:"spec,omitempty"`
	// Status defines the observed state of ThanosReceive
	Status ThanosReceiveStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ThanosReceiveList contains a list of ThanosReceive
type ThanosReceiveList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ThanosReceive `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ThanosReceive{}, &ThanosReceiveList{})
}
