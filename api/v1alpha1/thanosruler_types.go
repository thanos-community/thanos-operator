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

// ThanosRulerSpec defines the desired state of ThanosRuler
type ThanosRulerSpec struct {
	CommonThanosFields `json:",inline"`
	// Replicas is the number of Ruler replicas.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	// +kubebuilder:validation:Required
	Replicas int32 `json:"replicas,omitempty"`
	// By default, the operator will add all discoverable Queriers to the Ruler,
	// if they have query labels. You can optionally choose to override default
	// Query selector labels, to select a subset of Queries to query.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:={matchLabels:{"operator.thanos.io/query-api": "true"}}
	QueryLabelSelector *metav1.LabelSelector `json:"queryLabelSelector,omitempty"`
	// ObjectStorageConfig is the secret that contains the object storage configuration for Ruler to upload blocks.
	// +kubebuilder:validation:Required
	ObjectStorageConfig ObjectStorageConfig `json:"defaultObjectStorageConfig,omitempty"`
	// The operator will mount all ConfigMaps with the label operator.thanos.io/rule-file=true
	// to the Ruler as rule file. This field is set by default but you can choose to override
	// the default Rule Config selector label.
	// +kubebuilder:validation:Required
	// +kubebuilder:default:={matchLabels:{"operator.thanos.io/rule-file": "true"}}
	RuleConfigSelector *metav1.LabelSelector `json:"ruleConfigSelector,omitempty"`
	// AlertmanagerURL is the URL of the Alertmanager to which the Ruler will send alerts.
	// The scheme should not be empty e.g http might be used. The scheme may be prefixed with
	// 'dns+' or 'dnssrv+' to detect Alertmanager IPs through respective DNS lookups.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^((dns\+)?(dnssrv\+)?(http|https):\/\/)[a-zA-Z0-9\-\.]+\.[a-zA-Z]{2,}(:[0-9]{1,5})?$`
	AlertmanagerURL string `json:"alertmanagerURL,omitempty"`
	// ExternalLabels set on Ruler TSDB, for query time deduplication.
	// +kubebuilder:default={rule_replica: "$(NAME)"}
	// +kubebuilder:validation:Required
	ExternalLabels ExternalLabels `json:"externalLabels,omitempty"`
	// EvaluationInterval is the default interval at which rules are evaluated.
	// +kubebuilder:default="1m"
	EvaluationInterval Duration `json:"evaluationInterval,omitempty"`
	// Labels to drop before Ruler sends alerts to alertmanager.
	// +kubebuilder:validation:Optional
	AlertLabelDrop []string `json:"alertLabelDrop,omitempty"`
	// Retention is the duration for which the Thanos Rule StatefulSet will retain data.
	// +kubebuilder:default="2h"
	// +kubebuilder:validation:Required
	Retention Duration `json:"retention,omitempty"`
	// StorageSize is the size of the storage to be used by the Thanos Ruler StatefulSet.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$`
	StorageSize string `json:"storageSize"`
	// Additional configuration for the Thanos components. Allows you to add
	// additional args, containers, volumes, and volume mounts to Thanos Deployments,
	// and StatefulSets. Ideal to use for things like sidecars.
	// +kubebuilder:validation:Optional
	Additional `json:",inline"`
}

// TODO(saswatamcode): Add stateless mode

// ThanosRulerStatus defines the observed state of ThanosRuler
type ThanosRulerStatus struct {
	// Conditions represent the latest available observations of the state of the Ruler.
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ThanosRuler is the Schema for the thanosrulers API
type ThanosRuler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ThanosRulerSpec   `json:"spec,omitempty"`
	Status ThanosRulerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ThanosRulerList contains a list of ThanosRuler
type ThanosRulerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ThanosRuler `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ThanosRuler{}, &ThanosRulerList{})
}
