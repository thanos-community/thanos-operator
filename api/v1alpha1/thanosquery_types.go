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

// ThanosQuerySpec defines the desired state of ThanosQuery
type ThanosQuerySpec struct {
	CommonFields `json:",inline"`
	// Replicas is the number of querier replicas.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	// +kubebuilder:validation:Required
	Replicas int32 `json:"replicas,omitempty"`
	// Labels are additional labels to add to the Querier component.
	// +kubebuilder:validation:Optional
	Labels map[string]string `json:"labels,omitempty"`
	// ReplicaLabels are labels to treat as a replica indicator along which data is deduplicated.
	// Data can still be queried without deduplication using 'dedup=false' parameter.
	// Data includes time series, recording rules, and alerting rules.
	// Refer to https://thanos.io/tip/components/query.md/#deduplication-replica-labels
	// +kubebuilder:default:={"replica"}
	// +kubebuilder:validation:Optional
	ReplicaLabels []string `json:"replicaLabels,omitempty"`
	// StoreLabelSelector enables adding additional labels to build a custom label selector
	// for discoverable StoreAPIs. Values provided here will be appended to the default which are
	// {"operator.thanos.io/store-api": "true", "app.kubernetes.io/part-of": "thanos"}.
	// +kubebuilder:validation:Optional
	StoreLabelSelector *metav1.LabelSelector `json:"customStoreLabelSelector,omitempty"`
	// QueryFrontend is the configuration for the Query Frontend
	// If you specify this, the operator will create a Query Frontend in front of your query deployment.
	// +kubebuilder:validation:Optional
	QueryFrontend *QueryFrontendSpec `json:"queryFrontend,omitempty"`
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

// QueryFrontendSpec defines the desired state of ThanosQueryFrontend
type QueryFrontendSpec struct {
	CommonFields `json:",inline"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas,omitempty"`
	// CompressResponses enables response compression
	// +kubebuilder:default=true
	CompressResponses bool `json:"compressResponses,omitempty"`
	// By default, the operator will add the first discoverable Query API to the
	// Query Frontend, if they have query labels. You can optionally choose to override default
	// Query selector labels, to select a subset of QueryAPIs to query.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:={matchLabels:{"operator.thanos.io/query-api": "true"}}
	QueryLabelSelector *metav1.LabelSelector `json:"queryLabelSelector,omitempty"`
	// LogQueriesLongerThan sets the duration threshold for logging long queries
	// +kubebuilder:validation:Optional
	LogQueriesLongerThan *Duration `json:"logQueriesLongerThan,omitempty"`
	// QueryRangeResponseCacheConfig holds the configuration for the query range response cache
	// +kubebuilder:validation:Optional
	QueryRangeResponseCacheConfig *CacheConfig `json:"queryRangeResponseCacheConfig,omitempty"`
	// QueryRangeSplitInterval sets the split interval for query range
	// +kubebuilder:validation:Optional
	QueryRangeSplitInterval *Duration `json:"queryRangeSplitInterval,omitempty"`
	// LabelsSplitInterval sets the split interval for labels
	// +kubebuilder:validation:Optional
	LabelsSplitInterval *Duration `json:"labelsSplitInterval,omitempty"`
	// QueryRangeMaxRetries sets the maximum number of retries for query range requests
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=5
	QueryRangeMaxRetries int `json:"queryRangeMaxRetries,omitempty"`
	// LabelsMaxRetries sets the maximum number of retries for label requests
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=5
	LabelsMaxRetries int `json:"labelsMaxRetries,omitempty"`
	// LabelsDefaultTimeRange sets the default time range for label queries
	// +kubebuilder:validation:Optional
	LabelsDefaultTimeRange *Duration `json:"labelsDefaultTimeRange,omitempty"`
	// Additional configuration for the Thanos components
	Additional `json:",inline"`
}

// ThanosQueryStatus defines the observed state of ThanosQuery
type ThanosQueryStatus struct {
	// Conditions represent the latest available observations of the state of the Querier.
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ThanosQuery is the Schema for the thanosqueries API
type ThanosQuery struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ThanosQuerySpec   `json:"spec,omitempty"`
	Status ThanosQueryStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ThanosQueryList contains a list of ThanosQuery
type ThanosQueryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ThanosQuery `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ThanosQuery{}, &ThanosQueryList{})
}
