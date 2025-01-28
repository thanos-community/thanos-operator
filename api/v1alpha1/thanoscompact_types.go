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

// ThanosCompactSpec defines the desired state of ThanosCompact
type ThanosCompactSpec struct {
	// CommonFields are the options available to all Thanos components.
	CommonFields `json:",inline"`
	// Labels are additional labels to add to the Compact component.
	// +kubebuilder:validation:Optional
	Labels map[string]string `json:"labels,omitempty"`
	// ObjectStorageConfig is the object storage configuration for the compact component.
	// +kubebuilder:validation:Required
	ObjectStorageConfig ObjectStorageConfig `json:"objectStorageConfig"`
	// StorageSize is the size of the storage to be used by the Thanos Compact StatefulSets.
	// +kubebuilder:validation:Required
	StorageSize StorageSize `json:"storageSize"`
	// RetentionConfig is the retention configuration for the compact component.
	// +kubebuilder:validation:Required
	RetentionConfig RetentionResolutionConfig `json:"retentionConfig,omitempty"`
	// BlockConfig defines settings for block handling.
	// +kubebuilder:validation:Optional
	BlockConfig *BlockConfig `json:"blockConfig,omitempty"`
	// BlockViewerGlobalSync is the configuration for syncing the blocks between local and remote view for /global Block Viewer UI.
	// +kubebuilder:validation:Optional
	BlockViewerGlobalSync *BlockViewerGlobalSyncConfig `json:"blockViewerGlobalSync,omitempty"`
	// ShardingConfig is the sharding configuration for the compact component.
	// +kubebuilder:validation:Optional
	ShardingConfig *ShardingConfig `json:"shardingConfig,omitempty"`
	// CompactConfig is the configuration for the compact component.
	// +kubebuilder:validation:Optional
	CompactConfig *CompactConfig `json:"compactConfig,omitempty"`
	// DownsamplingConfig is the downsampling configuration for the compact component.
	// +kubebuilder:validation:Optional
	DownsamplingConfig *DownsamplingConfig `json:"downsamplingConfig,omitempty"`
	// DebugConfig is the debug configuration for the compact component.
	// +kubebuilder:validation:Optional
	DebugConfig *DebugConfig `json:"debugConfig,omitempty"`
	// Minimum time range to serve. Any data earlier than this lower time range will be ignored.
	// If not set, will be set as zero value, so most recent blocks will be served.
	// +kubebuilder:validation:Optional
	MinTime *Duration `json:"minTime,omitempty"`
	// Maximum time range to serve. Any data after this upper time range will be ignored.
	// If not set, will be set as max value, so all blocks will be served.
	// +kubebuilder:validation:Optional
	MaxTime *Duration `json:"maxTime,omitempty"`
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

// ThanosCompactStatus defines the observed state of ThanosCompact
type ThanosCompactStatus struct {
	// Conditions represent the latest available observations of the state of the hashring.
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// BlockViewerGlobalSyncConfig is the configuration for syncing the blocks between local and remote view for /global Block Viewer UI.
type BlockViewerGlobalSyncConfig struct {
	// BlockViewerGlobalSyncInterval for syncing the blocks between local and remote view for /global Block Viewer UI.
	// +kubebuilder:default="1m"
	// +kubebuilder:validation:Optional
	BlockViewerGlobalSyncInterval *Duration `json:"blockViewerGlobalSync,omitempty"`
	// BlockViewerGlobalSyncTimeout is the maximum time for syncing the blocks
	// between local and remote view for /global Block Viewer UI.
	// +kubebuilder:default="5m"
	// +kubebuilder:validation:Optional
	BlockViewerGlobalSyncTimeout *Duration `json:"blockViewerGlobalSyncTimeout,omitempty"`
}

type CompactConfig struct {
	// CompactConcurrency is the number of goroutines to use when compacting blocks.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Optional
	CompactConcurrency *int32 `json:"compactConcurrency,omitempty"`
	// BlockFetchConcurrency is the number of goroutines to use when fetching blocks from object storage.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Optional
	BlockFetchConcurrency *int32 `json:"blockFetchConcurrency,omitempty"`
	// CleanupInterval configures how often we should clean up partially uploaded blocks and blocks
	// that are marked for deletion.
	// Cleaning happens at the end of an iteration.
	// Setting this to 0s disables the cleanup.
	// +kubebuilder:default="5m"
	// +kubebuilder:validation:Optional
	CleanupInterval *Duration `json:"cleanupInterval,omitempty"`
	// ConsistencyDelay is the minimum age of fresh (non-compacted) blocks before they are being processed.
	// Malformed blocks older than the maximum of consistency-delay and 48h0m0s will be removed.
	// +kubebuilder:default="30m"
	// +kubebuilder:validation:Optional
	ConsistencyDelay *Duration `json:"blockConsistencyDelay,omitempty"`
}

type DebugConfig struct {
	// AcceptMalformedIndex allows compact to accept blocks with malformed index.
	// +kubebuilder:default=false
	// +kubebuilder:validation:Optional
	AcceptMalformedIndex *bool `json:"acceptMalformedIndex,omitempty"`
	// MaxCompactionLevel is the maximum compaction level to use when compacting blocks.
	// +kubebuilder:default=5
	// +kubebuilder:validation:Optional
	MaxCompactionLevel *int32 `json:"maxCompactionLevel,omitempty"`
	// HaltOnError halts the compact process on critical compaction error.
	// +kubebuilder:default=false
	// +kubebuilder:validation:Optional
	HaltOnError *bool `json:"haltOnError,omitempty"`
}

// DownsamplingConfig defines the downsampling configuration for the compact component.
type DownsamplingConfig struct {
	// Disable downsampling.
	// +kubebuilder:default=false
	Disable *bool `json:"downsamplingEnabled,omitempty"`
	// Concurrency is the number of goroutines to use when downsampling blocks.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Optional
	Concurrency *int32 `json:"downsamplingConcurrency,omitempty"`
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
	// +listType=map
	// +listMapKey=shardName
	ExternalLabelSharding []ExternalLabelShardingConfig `json:"externalLabelSharding,omitempty"`
}

// ExternalLabelShardingConfig defines the sharding configuration based on explicit external labels and their values.
// The keys are the external labels to shard on and the values are the values (as regular expressions) to shard on.
// Each value will be a configured and deployed as a separate compact component.
// For example, if the 'label' is set to `tenant_id` with values `tenant-a` and `!tenant-a`
// two compact components will be deployed.
// The resulting compact StatefulSets will have an appropriate --selection.relabel-config flag set to the value of the external label sharding.
// And named such that:
//
//		The first compact component will have the name {ThanosCompact.Name}-{shardName}-0 with the flag
//	    --selector.relabel-config=
//	       - source_labels:
//	         - tenant_id
//	         regex: 'tenant-a'
//	         action: keep
//
//		The second compact component will have the name {ThanosCompact.Name}-{shardName}-1 with the flag
//	    --selector.relabel-config=
//	       - source_labels:
//	         - tenant_id
//	         regex: '!tenant-a'
//	         action: keep
type ExternalLabelShardingConfig struct {
	// ShardName is the name of the shard.
	// ShardName is used to identify the shard in the compact component.
	// +kubebuilder:validation:Required
	ShardName string `json:"shardName"`
	// Label is the external label to shard on.
	// +kubebuilder:validation:Required
	Label string `json:"label"`
	// Values are the values (as regular expressions) to shard on.
	Values []string `json:"values"`
}

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
