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
	// CommonFields are the options available to all Thanos components.
	// +kubebuilder:validation:Optional
	CommonFields `json:",inline"`
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
	// ExternalLabels set and forwarded by the router to the ingesters.
	// +kubebuilder:default={receive: "true"}
	// +kubebuilder:validation:Required
	ExternalLabels ExternalLabels `json:"externalLabels,omitempty"`
	// Additional configuration for the Thanos components. Allows you to add
	// additional args, containers, volumes, and volume mounts to Thanos Deployments,
	// and StatefulSets. Ideal to use for things like sidecars.
	// +kubebuilder:validation:Optional
	Additional `json:",inline"`
}

// IngesterSpec represents the configuration for the ingestor
type IngesterSpec struct {
	// DefaultObjectStorageConfig is the secret that contains the object storage configuration for the ingest components.
	// Can be overridden by the ObjectStorageConfig in the IngesterHashringSpec per hashring.
	// +kubebuilder:validation:Required
	DefaultObjectStorageConfig ObjectStorageConfig `json:"defaultObjectStorageConfig,omitempty"`
	// Hashrings is a list of hashrings to route to.
	// +kubebuilder:validation:MaxItems=100
	// +kubebuilder:validation:Required
	// +listType=map
	// +listMapKey=name
	Hashrings []IngesterHashringSpec `json:"hashrings,omitempty"`
	// Additional configuration for the Thanos components. Allows you to add
	// additional args, containers, volumes, and volume mounts to Thanos Deployments,
	// and StatefulSets. Ideal to use for things like sidecars.
	// +kubebuilder:validation:Optional
	Additional `json:",inline"`
}

// IngesterHashringSpec represents the configuration for a hashring to be used by the Thanos Receive StatefulSet.
type IngesterHashringSpec struct {
	// CommonFields are the options available to all Thanos components.
	// +kubebuilder:validation:Optional
	CommonFields `json:",inline"`
	// Name is the name of the hashring.
	// Name will be used to generate the names for the resources created for the hashring.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^$|^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	Name string `json:"name"`
	// Labels are additional labels to add to the ingester components.
	// Labels set here will overwrite the labels inherited from the ThanosReceive object if they have the same key.
	// +kubebuilder:validation:Optional
	Labels map[string]string `json:"labels,omitempty"`
	// ExternalLabels to add to the ingesters tsdb blocks.
	// +kubebuilder:default={replica: "$(POD_NAME)"}
	// +kubebuilder:validation:Required
	ExternalLabels ExternalLabels `json:"externalLabels,omitempty"`
	// Replicas is the number of replicas/members of the hashring to add to the Thanos Receive StatefulSet.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	// +kubebuilder:validation:Required
	Replicas int32 `json:"replicas,omitempty"`
	// TSDB configuration for the ingestor.
	// +kubebuilder:validation:Required
	TSDBConfig TSDBConfig `json:"tsdbConfig,omitempty"`
	// ObjectStorageConfig is the secret that contains the object storage configuration for the hashring.
	// +kubebuilder:validation:Optional
	ObjectStorageConfig *ObjectStorageConfig `json:"objectStorageConfig,omitempty"`
	// StorageSize is the size of the storage to be used by the Thanos Receive StatefulSet.
	// +kubebuilder:validation:Required
	StorageSize StorageSize `json:"storageSize"`
	// TenancyConfig is the configuration for the tenancy options.
	// +kubebuilder:validation:Optional
	TenancyConfig *TenancyConfig `json:"tenancyConfig,omitempty"`
	// AsyncForwardWorkerCount is the number of concurrent workers processing forwarding of remote-write requests.
	// +kubebuilder:default:=5
	// +kubebuilder:validation:Optional
	AsyncForwardWorkerCount *uint64 `json:"asyncForwardWorkerCount,omitempty"`
	// StoreLimitsOptions is the configuration for the store API limits options.
	// +kubebuilder:validation:Optional
	StoreLimitsOptions *StoreLimitsOptions `json:"storeLimitsOptions,omitempty"`
	// TooFarInFutureTimeWindow is the allowed time window for ingesting samples too far in the future.
	// 0s means disabled.
	// +kubebuilder:default:="0s"
	// +kubebuilder:validation:Optional
	TooFarInFutureTimeWindow *Duration `json:"tooFarInFutureTimeWindow,omitempty"`
}

// TenancyConfig is the configuration for the tenancy options.
type TenancyConfig struct {
	// Tenants is a list of tenants that should be matched by the hashring.
	// An empty list matches all tenants.
	// +kubebuilder:validation:Optional
	Tenants []string `json:"tenants,omitempty"`
	// TenantMatcherType is the type of tenant matching to use.
	// +kubebuilder:default:="exact"
	// +kubebuilder:validation:Enum=exact;glob
	TenantMatcherType string `json:"tenantMatcherType,omitempty"`
	// TenantHeader is the HTTP header to determine tenant for write requests.
	// +kubebuilder:default="THANOS-TENANT"
	TenantHeader string `json:"tenantHeader,omitempty"`
	// TenantCertificateField is the TLS client's certificate field to determine tenant for write requests.
	// +kubebuilder:validation:Enum=organization;organizationalUnit;commonName
	// +kubebuilder:validation:Optional
	TenantCertificateField *string `json:"tenantCertificateField,omitempty"`
	// DefaultTenantID is the default tenant ID to use when none is provided via a header.
	// +kubebuilder:default="default-tenant"
	DefaultTenantID string `json:"defaultTenantID,omitempty"`
	// SplitTenantLabelName is the label name through which the request will be split into multiple tenants.
	// +kubebuilder:validation:Optional
	SplitTenantLabelName *string `json:"splitTenantLabelName,omitempty"`
	// TenantLabelName is the label name through which the tenant will be announced.
	// +kubebuilder:default="tenant_id"
	TenantLabelName string `json:"tenantLabelName,omitempty"`
}

// ThanosReceiveSpec defines the desired state of ThanosReceive
// +kubebuilder:validation:XValidation:rule="self.ingesterSpec.hashrings.all(h, h.replicas >= self.routerSpec.replicationFactor )", message=" Ingester replicas must be greater than or equal to the Router replicas"
type ThanosReceiveSpec struct {
	// Router is the configuration for the router.
	// +kubebuilder:validation:Required
	Router RouterSpec `json:"routerSpec,omitempty"`
	// Ingester is the configuration for the ingestor.
	// +kubebuilder:validation:Required
	Ingester IngesterSpec `json:"ingesterSpec,omitempty"`
	// When a resource is paused, no actions except for deletion
	// will be performed on the underlying objects.
	// +kubebuilder:validation:Optional
	Paused *bool `json:"paused,omitempty"`
	// FeatureGates are feature gates for the compact component.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:={"serviceMonitor":{"enable":true}}
	FeatureGates *FeatureGates `json:"featureGates,omitempty"`
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
