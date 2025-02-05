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

// ThanosStoreSpec defines the desired state of ThanosStore
type ThanosStoreSpec struct {
	CommonFields `json:",inline"`
	// Replicas is the number of store or store shard replicas.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	// +kubebuilder:validation:Required
	Replicas int32 `json:"replicas,omitempty"`
	// Labels are additional labels to add to the Store component.
	// +kubebuilder:validation:Optional
	Labels map[string]string `json:"labels,omitempty"`
	// ObjectStorageConfig is the secret that contains the object storage configuration for Store Gateways.
	// +kubebuilder:validation:Required
	ObjectStorageConfig ObjectStorageConfig `json:"objectStorageConfig,omitempty"`
	// StorageSize is the size of the storage to be used by the Thanos Store StatefulSets.
	// +kubebuilder:validation:Required
	StorageSize StorageSize `json:"storageSize"`
	// Duration after which the blocks marked for deletion will be filtered out while fetching blocks.
	// The idea of ignore-deletion-marks-delay is to ignore blocks that are marked for deletion with some delay.
	// This ensures store can still serve blocks that are meant to be deleted but do not have a replacement yet.
	// If delete-delay duration is provided to compactor or bucket verify component, it will upload deletion-mark.json
	// file to mark after what duration the block should be deleted rather than deleting the block straight away.
	// +kubebuilder:default="24h"
	IgnoreDeletionMarksDelay Duration `json:"ignoreDeletionMarksDelay,omitempty"`
	// IndexCacheConfig allows configuration of the index cache.
	// See format details: https://thanos.io/tip/components/store.md/#index-cache
	// +kubebuilder:validation:Optional
	IndexCacheConfig *CacheConfig `json:"indexCacheConfig,omitempty"`
	// CachingBucketConfig allows configuration of the caching bucket.
	// See format details: https://thanos.io/tip/components/store.md/#caching-bucket
	// +kubebuilder:validation:Optional
	CachingBucketConfig *CacheConfig `json:"cachingBucketConfig,omitempty"`
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
	// StoreLimitsOptions allows configuration of the store API limits.
	// +kubebuilder:validation:Optional
	StoreLimitsOptions *StoreLimitsOptions `json:"storeLimitsOptions,omitempty"`
	// IndexHeaderConfig allows configuration of the Store Gateway index header.
	// +kubebuilder:validation:Optional
	IndexHeaderConfig *IndexHeaderConfig `json:"indexHeaderConfig,omitempty"`
	// BlockConfig defines settings for block handling.
	// +kubebuilder:validation:Optional
	BlockConfig *BlockConfig `json:"blockConfig,omitempty"`
	// When a resource is paused, no actions except for deletion
	// will be performed on the underlying objects.
	// +kubebuilder:validation:Optional
	Paused *bool `json:"paused,omitempty"`
	// FeatureGates are feature gates for the compact component.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:={"serviceMonitor":{"enable":true}}
	FeatureGates *FeatureGates `json:"featureGates,omitempty"`
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
	Shards int32 `json:"shards,omitempty"`
}

// IndexHeaderConfig allows configuration of the Store Gateway index header.
type IndexHeaderConfig struct {
	// If true, Store Gateway will lazy memory map index-header only once the block is required by a query.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	EnableLazyReader *bool `json:"enableLazyReader,omitempty"`
	// If index-header lazy reader is enabled and this idle timeout setting is > 0, memory map-ed index-headers will be automatically released after 'idle timeout' inactivity
	// +kubebuilder:default="5m"
	// +kubebuilder:validation:Optional
	LazyReaderIdleTimeout *Duration `json:"lazyReaderIdleTimeout,omitempty"`
	// Strategy of how to download index headers lazily.
	// If eager, always download index header during initial load. If lazy, download index header during query time.
	// +kubebuilder:validation:Enum=eager;lazy
	// +kubebuilder:default=eager
	// +kubebuilder:validation:Optional
	LazyDownloadStrategy *string `json:"lazyDownloadStrategy,omitempty"`
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
