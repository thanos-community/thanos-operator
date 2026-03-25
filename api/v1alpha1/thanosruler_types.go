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

// ThanosRulerSpec defines the desired state of ThanosRuler
type ThanosRulerSpec struct {
	CommonFields `json:",inline"`
	// StatefulSetFields are the options available to all Thanos stateful
	// components.
	StatefulSetFields `json:",inline"`
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
	// +kubebuilder:validation:Optional
	ObjectStorageConfig *ObjectStorageConfig `json:"objectStorageConfig,omitempty"`
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
	// RemoteWriteSpec defines the configuration to write samples from Prometheus to a remote endpoint
	// +kubebuilder:validation:Optional
	RemoteWriteSpec []RemoteWriteSpec `json:"remoteWriteSpec,omitempty"`
}

type RuleTenancyConfig struct {
	// EnforcedTenantIdentifier will be injected into each Prometheus rule as a label to enforce tenancy
	// For example if enforcedTenantIdentifier: "tenant_id" then up{} becomes up{tenant_id={TenantSpecifierLabelValue}
	// +kubebuilder:default "tenant_id"
	// +kubebuilder:validation:Optional
	EnforcedTenantIdentifier *string `json:"enforcedTenantIdentifier,omitempty"`
	// TenantSpecifierLabel is the key of the label of the ConfigMap or PrometheusRule that will be used to set the value of the EnforcedTenantIdentifier
	// +kubebuilder:default "operator.thanos.io/tenant"
	// +kubebuilder:validation:Optional
	TenantSpecifierLabel *string `json:"tenantSpecifierLabel,omitempty"`
}

type RemoteWriteSpec struct {
	// URL defines the URL of the endpoint to send samples to.
	// +kubebuilder:validation:Required
	URL string `json:"url"`
	// Name of the remote write queue, it must be unique if specified.
	// The name is used in metrics and logging in order to differentiate queues.
	// +kubebuilder:validation:Optional
	Name *string `json:"name,omitempty"`
	// MessageVersion defines the Remote Write message’s version to use when writing to the endpoint.
	// Version1.0 corresponds to the prometheus.WriteRequest protobuf message introduced in Remote Write 1.0.
	// Version2.0 corresponds to the io.prometheus.write.v2.Request protobuf message introduced in Remote Write 2.0.
	// When Version2.0 is selected, Prometheus will automatically be configured to append the metadata of scraped metrics to the WAL.
	// Before setting this field, consult with your remote storage provider what message version it supports.
	// +kubebuilder:validation:Optional
	MessageVersion *RemoteWriteMessageVersion `json:"messageVersion,omitempty"`
	// SendExemplars enables sending of exemplars over remote write.
	// Note that exemplar-storage itself must be enabled using the spec.enableFeatures option for exemplars to be scraped in the first place.
	// +kubebuilder:validation:Optional
	SendExemplars *bool `json:"sendExamples,omitempty"`
	// SendNativeHistograms enables sending of native histograms, also known as sparse histograms over remote write.
	// +kubebuilder:validation:Optional
	SendNativeHistograms *bool `json:"sendNativeHistograms,omitempty"`
	// RemoteTimeout defines the timeout for requests to the remote write endpoint.
	// +kubebuilder:validation:Optional
	RemoteTimeout *Duration `json:"remoteTimeout,omitempty"`
	// Headers defines the custom HTTP headers to be sent along with each remote write request.
	// Be aware that headers that are set by Prometheus itself can’t be overwritten.
	// +kubebuilder:validation:Optional
	Headers *map[string]string `json:"headers,omitempty"`
	// ProxyURL defines the HTTP proxy server to use.
	// +kubebuilder:validation:Optional
	ProxyURL *string `json:"proxyUrl,omitempty"`
	// noProxy defines a comma-separated string that can contain IPs, CIDR notation, domain names that should be excluded from proxying.
	// IP and domain names can contain port numbers.
	// +kubebuilder:validation:Optional
	NoProxy *string `json:"noProxy,omitempty"`
	// ProxyFromEnvironment defines whether to use the proxy configuration defined by environment variables (HTTP_PROXY, HTTPS_PROXY, and NO_PROXY).
	// +kubebuilder:validation:Optional
	ProxyFromEnvironment *bool `json:"proxyFromEnvironment,omitempty"`
	// ProxyConnectHeader optionally specifies headers to send to proxies during CONNECT requests.
	// +kubebuilder:validation:Optional
	ProxyConnectHeader *map[string]corev1.SecretKeySelector `json:"proxyConnectHeader,omitempty"`
	// FollowRedirects defines whether HTTP requests follow HTTP 3xx redirects.
	// +kubebuilder:validation:Optional
	FollowRedirects *bool `json:"followRedirects,omitempty"`
	// QueueConfig allows tuning of the remote write queue parameters.
	// +kubebuilder:validation:Optional
	QueueConfig *QueueConfig `json:"queueConfig,omitempty"`
}

type QueueConfig struct{}

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
