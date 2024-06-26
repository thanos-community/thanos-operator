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
	CommonThanosFields `json:",inline"`
	// Replicas is the number of querier replicas.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	// +kubebuilder:validation:Required
	Replicas int32 `json:"replicas,omitempty"`
	// Labels are additional labels to add to the Querier component.
	// +kubebuilder:validation:Optional
	Labels map[string]string `json:"labels,omitempty"`
	// Labels to treat as a replica indicator along which data is deduplicated.
	// Still you will be able to query without deduplication using 'dedup=false' parameter.
	// Data includes time series, recording rules, and alerting rules.
	// Refer to https://thanos.io/tip/components/query.md/#deduplication-replica-labels
	// +kubebuilder:default:={"replica"}
	// +kubebuilder:validation:Optional
	QuerierReplicaLabels []string `json:"querierReplicaLabels,omitempty"`
	// By default, the operator will add all discoverable StoreAPIs to the Querier,
	// if they have store labels. You can optionally choose to override default
	// StoreAPI selector labels, to select a subset of StoreAPIs to query.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:={matchLabels:{"operator.thanos.io/store-api": "true"}}
	StoreLabelSelector *metav1.LabelSelector `json:"customStoreLabelSelector,omitempty"`
	// Additional configuration for the Thanos components. Allows you to add
	// additional args, containers, volumes, and volume mounts to Thanos Deployments,
	// and StatefulSets. Ideal to use for things like sidecars.
	// +kubebuilder:validation:Optional
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
