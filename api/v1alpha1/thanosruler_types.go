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
	CommonFields `json:",inline"`
	// StatefulSetFields are the options available to all Thanos stateful
	// components.
	StatefulSetFields `json:",inline"`
	// Labels are additional labels to add to the Ruler component.
	// +kubebuilder:validation:Optional
	Labels map[string]string `json:"labels,omitempty"`
	// Replicas is the number of Ruler replicas.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	// +kubebuilder:validation:Required
	Replicas int32 `json:"replicas,omitempty"`
	// QueryLabelSelector is the label selector to discover Queriers.
	// It enables adding additional labels to build a custom label selector for discoverable QueryAPIs.
	// Values provided here will be appended to the default which are:
	// {"operator.thanos.io/query-api": "true", "app.kubernetes.io/part-of": "thanos"}.
	// +kubebuilder:validation:Optional
	QueryLabelSelector *metav1.LabelSelector `json:"queryLabelSelector,omitempty"`
	// ObjectStorageConfig is the secret that contains the object storage configuration for Ruler to upload blocks.
	// +kubebuilder:validation:Required
	ObjectStorageConfig ObjectStorageConfig `json:"defaultObjectStorageConfig,omitempty"`
	// RuleConfigSelector is the label selector to discover ConfigMaps with rule files.
	// It also discovers PrometheusRule CustomResources if the feature flag is enabled.
	// PrometheusRules are converted them into ConfigMaps with rule files internally.
	// It enables adding additional labels to build a custom label selector for discoverable rule files.
	// Values provided here will be appended to the default which is: operator.thanos.io/prometheus-rule: "true"
	// +kubebuilder:default:={matchLabels:{"operator.thanos.io/prometheus-rule": "true"}}
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self.matchLabels.size() >= 1 || self.matchExpressions.size() >= 1",message="ruleConfigSelector must have at least one label selector"
	RuleConfigSelector metav1.LabelSelector `json:"ruleConfigSelector,omitempty"`
	// AlertmanagerURL is the URL of the Alertmanager to which the Ruler will send alerts.
	// The scheme should not be empty e.g http might be used. The scheme may be prefixed with
	// 'dns+' or 'dnssrv+' to detect Alertmanager IPs through respective DNS lookups.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^((dns\+)?(dnssrv\+)?(http|https):\/\/)[a-zA-Z0-9\-\.]+\.[a-zA-Z]{2,}(:[0-9]{1,5})?$`
	AlertmanagerURL string `json:"alertmanagerURL,omitempty"` //nolint:tagliatelle
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
	// StorageConfiguration represents the storage to be used by the Thanos Ruler StatefulSets.
	// +kubebuilder:validation:Required
	StorageConfiguration StorageConfiguration `json:"storage"`
	// When a resource is paused, no actions except for deletion
	// will be performed on the underlying objects.
	// +kubebuilder:validation:Optional
	Paused *bool `json:"paused,omitempty"`
	// RuleTenancyConfig is the configuration for the rule tenancy.
	// +kubebuilder:validation:Optional
	RuleTenancyConfig *RuleTenancyConfig `json:"ruleTenancyConfig,omitempty"`
	// Additional configuration for the Thanos components. Allows you to add
	// additional args, containers, volumes, and volume mounts to Thanos Deployments,
	// and StatefulSets. Ideal to use for things like sidecars.
	// +kubebuilder:validation:Optional
	Additional `json:",inline"`
}

type RuleTenancyConfig struct {
	// EnforcedTenantIdentifier will be injected into each Prometheus rule as a label to enforce tenancy
	// For example if  enforcedTenantIdentifier: tenant_id  then up{} becomes up{tenant_id={valueUnderneathStillToBeNamedProperly}
	// default "tenant_id"
	EnforcedTenantIdentifier string `json:"EnforcedTenantIdentifier,omitempty"`
	// TenantSpecifierLabel is the key of the label of the ConfigMapPrometheusRule that will be used to set the value of the EnforcedTenantIdentifier
	// default `operator.thanos.io/tenant`
	// +kubebuilder:validation:Optional
	TenantSpecifierLabel string `json:"tenantSpecifierLabel,omitempty"`
}

// TODO(saswatamcode): Add stateless mode

// ThanosRulerStatus defines the observed state of ThanosRuler
type ThanosRulerStatus struct {
	// Conditions represent the latest available observations of the state of the Ruler.
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
	// Paused is a flag that indicates if the Ruler is paused.
	// +kubebuilder:validation:Optional
	Paused            *bool `json:"paused,omitempty"`
	StatefulSetStatus `json:",inline"`
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
