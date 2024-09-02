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

// ThanosQueryFrontendSpec defines the desired state of ThanosQueryFrontend
type ThanosQueryFrontendSpec struct {
	CommonThanosFields `json:",inline"`
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
	QueryRangeResponseCacheConfig *corev1.ConfigMapKeySelector `json:"queryRangeResponseCacheConfig,omitempty"`
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

// ThanosQueryFrontendStatus defines the observed state of ThanosQueryFrontend
type ThanosQueryFrontendStatus struct {
	// Conditions represent the latest available observations of the state of the Query Frontend.
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ThanosQueryFrontend is the Schema for the thanosqueryfrontends API
type ThanosQueryFrontend struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ThanosQueryFrontendSpec   `json:"spec,omitempty"`
	Status ThanosQueryFrontendStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ThanosQueryFrontendList contains a list of ThanosQueryFrontend
type ThanosQueryFrontendList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ThanosQueryFrontend `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ThanosQueryFrontend{}, &ThanosQueryFrontendList{})
}
