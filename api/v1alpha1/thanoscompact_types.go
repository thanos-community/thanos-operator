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

// BlockDiscoveryStrategy represents the strategy to use for block discovery.
type BlockDiscoveryStrategy string

const (
	// BlockDiscoveryStrategyConcurrent means stores will concurrently issue one call
	// per directory to discover active blocks storage.
	BlockDiscoveryStrategyConcurrent BlockDiscoveryStrategy = "concurrent"
	// BlockDiscoveryStrategyRecursive means stores iterate through all objects in storage
	// recursively traversing into each directory.
	// This avoids N+1 calls at the expense of having slower bucket iterations.
	BlockDiscoveryStrategyRecursive BlockDiscoveryStrategy = "recursive"
)

// ThanosCompactSpec defines the desired state of ThanosCompact
type ThanosCompactSpec struct {
	// CommonThanosFields are the options available to all Thanos components.
	CommonThanosFields `json:",inline"`
	// ObjectStorageConfig is the object storage configuration for the compact component.
	// +kubebuilder:validation:Required
	ObjectStorageConfig *ObjectStorageConfig `json:"objectStorageConfig,omitempty"`
	// RetentionConfig is the retention configuration for the compact component.
	// +kubebuilder:validation:Required
	RetentionConfig RetentionResolutionConfig `json:"retentionConfig,omitempty"`
	// BlockConfig defines settings for block handling.
	// +kubebuilder:validation:Optional
	BlockConfig BlockConfig `json:"blockConfig,omitempty"`
	// GroupConfig defines settings for group handling.
	// +kubebuilder:validation:Optional
	GroupConfig GroupConfig `json:"groupConfig,omitempty"`
	// ShardingConfig is the sharding configuration for the compact component.
	// +kubebuilder:validation:Optional
	ShardingConfig *ShardingConfig `json:"shardingConfig,omitempty"`
}

// ThanosCompactStatus defines the observed state of ThanosCompact
type ThanosCompactStatus struct {
	// Conditions represent the latest available observations of the state of the hashring.
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// BlockConfig defines settings for block handling.
type BlockConfig struct {
	// BlockDiscoveryStrategy is the discovery strategy to use for block discovery in storage.
	// +kubebuilder:default="concurrent"
	// +kubebuilder:validation:Enum=concurrent;recursive
	BlockDiscoveryStrategy BlockDiscoveryStrategy `json:"blockDiscoveryStrategy,omitempty"`
	// BlockFilesConcurrency is the number of goroutines to use when to use when
	// fetching/uploading block files from object storage.
	// +kubebuilder:default=1
	BlockFilesConcurrency int32 `json:"blockFilesConcurrency,omitempty"`
	// BlockMetaFetchConcurrency is the number of goroutines to use when fetching block metadata from object storage.
	// +kubebuilder:default=32
	BlockMetaFetchConcurrency int32 `json:"blockMetaFetchConcurrency,omitempty"`
	// BlockViewerGlobalSyncInterval for syncing the blocks between local and remote view for /global Block Viewer UI.
	// +kubebuilder:default="1m"
	BlockViewerGlobalSyncInterval Duration `json:"blockViewerGlobalSync,omitempty"`
	// BlockViewerGlobalSyncTimeout is the maximum time for syncing the blocks
	// between local and remote view for /global Block Viewer UI.
	// +kubebuilder:default="5m"
	BlockViewerGlobalSyncTimeout Duration `json:"blockViewerGlobalSyncTimeout,omitempty"`
	// BlockFetchConcurrency is the number of goroutines to use when fetching blocks from object storage.
	// +kubebuilder:default=1
	BlockFetchConcurrency int32 `json:"blockFetchConcurrency,omitempty"`
	// BlockConsistencyDelay is the minimum age of fresh (non-compacted) blocks before they are being processed.
	// Malformed blocks older than the maximum of consistency-delay and 48h0m0s will be removed.
	// +kubebuilder:default="30m"
	BlockConsistencyDelay Duration `json:"blockConsistencyDelay,omitempty"`
	// BlockCleanupInterval configures how often we should clean up partially uploaded blocks and blocks
	// that are marked for deletion.
	// Cleaning happens at the end of an iteration.
	// Setting this to 0s disables the cleanup.
	// +kubebuilder:default="5m"
	BlockCleanupInterval Duration `json:"blockCleanupInterval,omitempty"`
}

// GroupConfig defines settings for group handling.
type GroupConfig struct {
	// BlockDiscoveryStrategy is the discovery strategy to use for block discovery in storage.
	// +kubebuilder:default="concurrent"
	// +kubebuilder:validation:Enum=concurrent;recursive
	BlockDiscoveryStrategy BlockDiscoveryStrategy `json:"blockDiscoveryStrategy,omitempty"`
	// BlockFilesConcurrency is the number of goroutines to use when to use when
	// fetching/uploading block files from object storage.
	// +kubebuilder:default=1
	BlockFilesConcurrency int32 `json:"blockFilesConcurrency,omitempty"`
	// BlockMetaFetchConcurrency is the number of goroutines to use when fetching block metadata from object storage.
	// +kubebuilder:default=32
	BlockMetaFetchConcurrency int32 `json:"blockMetaFetchConcurrency,omitempty"`
	// BlockViewerGlobalSyncInterval for syncing the blocks between local and remote view for /global Block Viewer UI.
	// +kubebuilder:default="1m"
	BlockViewerGlobalSyncInterval Duration `json:"blockViewerGlobalSync,omitempty"`
	// BlockViewerGlobalSyncTimeout is the maximum time for syncing the blocks
	// between local and remote view for /global Block Viewer UI.
	// +kubebuilder:default="5m"
	BlockViewerGlobalSyncTimeout Duration `json:"blockViewerGlobalSyncTimeout,omitempty"`
	// GroupConcurrency is the number of goroutines to use when compacting groups.
	// +kubebuilder:default=1
	GroupConcurrency int32 `json:"groupConcurrency,omitempty"`
}

// DownsamplingConfig defines the downsampling configuration for the compact component.
type DownsamplingConfig struct {
	// Enabled is a flag to enable downsampling.
	// +kubebuilder:default=true
	// +kubebuilder:validation:Optional
	Enabled bool `json:"enabled,omitempty"`
	// Concurrency is the number of goroutines to use when downsampling blocks.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Optional
	Concurrency int32 `json:"concurrency,omitempty"`
}

// RetentionResolutionConfig defines the retention configuration for the compact component.
type RetentionResolutionConfig struct {
	// Raw is the retention configuration for the raw samples.
	// This configures how long to retain raw samples in the storage.
	// The default value is 0d, which means samples are retained indefinitely.
	// +kubebuilder:default="0d"
	// +kubebuilder:validation:Required
	Raw Duration `json:"raw,omitempty"`
	// FiveMinutes is the retention configuration for samples of resolution 1 (5 minutes).
	// This configures how long to retain samples of resolution 1 (5 minutes) in storage.
	// The default value is 0d, which means these samples are retained indefinitely.
	// +kubebuilder:default="0d"
	// +kubebuilder:validation:Required
	FiveMinutes Duration `json:"fiveMinutes,omitempty"`
	// OneHour is the retention configuration for samples of resolution 2 (1 hour).
	// This configures how long to retain samples of resolution 2 (1 hour) in storage.
	// The default value is 0d, which means these samples are retained indefinitely.
	// +kubebuilder:default="0d"
	// +kubebuilder:validation:Required
	OneHour Duration `json:"oneHour,omitempty"`
}

// ShardingConfig defines the sharding configuration for the compact component.
type ShardingConfig struct {
	// ExternalLabelSharding is the sharding configuration based on explicit external labels and their values.
	// +kubebuilder:validation:Optional
	ExternalLabelSharding ExternalLabelShardingConfig `json:"externalLabelSharding,omitempty"`
}

// ExternalLabelShardingConfig defines the sharding configuration based on explicit external labels and their values.
// The keys are the external labels to shard on and the values are the values (as regular expressions) to shard on.
// Each value will be a configured and deployed as a separate compact component.
type ExternalLabelShardingConfig map[string]string

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ThanosCompact is the Schema for the thanoscompacts API
type ThanosCompact struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ThanosCompactSpec   `json:"spec,omitempty"`
	Status ThanosCompactStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ThanosCompactList contains a list of ThanosCompact
type ThanosCompactList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ThanosCompact `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ThanosCompact{}, &ThanosCompactList{})
}
