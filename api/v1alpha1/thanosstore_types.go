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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ThanosStoreSpec defines the desired state of ThanosStore
type ThanosStoreSpec struct {
	CommonThanosFields `json:",inline"`
	// ObjectStorageConfig is the secret that contains the object storage configuration for Store Gateways.
	// +kubebuilder:validation:Required
	ObjectStorageConfig ObjectStorageConfig `json:"objectStorageConfig,omitempty"`
	// Duration after which the blocks marked for deletion will be filtered out while fetching blocks.
	// The idea of ignore-deletion-marks-delay is to ignore blocks that are marked for deletion with some delay.
	// This ensures store can still serve blocks that are meant to be deleted but do not have a replacement yet.
	// If delete-delay duration is provided to compactor or bucket verify component, it will upload deletion-mark.json
	// file to mark after what duration the block should be deleted rather than deleting the block straight away.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="24h"
	IgnoreDeletionMarksDelay *Duration `json:"ignoreDeletionMarksDelay,omitempty"`
	// YAML file that contains index cache configuration. See format details: https://thanos.io/tip/components/store.md/#index-cache
	// IN-MEMORY config is loaded by default if not specified.
	// +kubebuilder:validation:Optional
	IndexCacheConfig corev1.ConfigMapKeySelector `json:"indexCacheConfig,omitempty"`
	// YAML that contains configuration for caching bucket.
	// See format details: https://thanos.io/tip/components/store.md/#caching-bucket"
	// IN-MEMORY config is loaded by default if not specified.
	// +kubebuilder:validation:Optional
	CachingBucketConfig corev1.ConfigMapKeySelector `json:"cachingBucketConfig,omitempty"`
	// ShardingStrategy defines the sharding strategy for the Store Gateways across object storage blocks.
	// +kubebuilder:validation:Required
	ShardingStrategy ShardingStrategy `json:"shardingStrategy,omitempty"`
	// Minimum time range to serve. Any data earlier than this lower time range will be ignored.
	// If not set, will be set as zero value, so most recent blocks will be served.
	// +kubebuilder:validation:Optional
	MinTime *Duration `json:"minTime,omitempty"`
	// Maximum time range to serve. Any data after this upper time range will be ignored.
	// If not set, will be set as max value, so all blocks will be served.
	// +kubebuilder:validation:Optional
	MaxTime *Duration `json:"maxTime,omitempty"`
	// Additional configuration for the Thanos components. Allows you to add
	// additional args, containers, volumes, and volume mounts to Thanos Deployments,
	// and StatefulSets. Ideal to use for things like sidecars.
	// +kubebuilder:validation:Optional
	Additional `json:",inline"`
}

type ShardingStrategyType string

const (
	// Block is the block modulo sharding strategy for sharding Stores according to block ids.
	Block ShardingStrategyType = "block"
)

// ShardingStrategy controls the automatic deployment of multiple store gateways sharded by block ID
// by hashmoding __block_id label value.
type ShardingStrategy struct {
	// Type here is the type of sharding strategy.
	// +kubebuilder:validation:Required
	// +kubebuilder:default="block"
	// +kubebuilder:validation:Enum=block
	Type ShardingStrategyType `json:"type,omitempty"`
	// Shards is the number of shards to split the data into.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	// +kubebuilder:validation:Optional
	Shards *int32 `json:"shards,omitempty"`
	// ReplicaPerShard is the number of replicas per shard.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	// +kubebuilder:validation:Optional
	ShardReplicas *int32 `json:"shardReplicas,omitempty"`
}

// ThanosStoreStatus defines the observed state of ThanosStore
type ThanosStoreStatus struct {
	// Conditions represent the latest available observations of the state of the Querier.
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ThanosStore is the Schema for the thanosstores API
type ThanosStore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ThanosStoreSpec   `json:"spec,omitempty"`
	Status ThanosStoreStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ThanosStoreList contains a list of ThanosStore
type ThanosStoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ThanosStore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ThanosStore{}, &ThanosStoreList{})
}
