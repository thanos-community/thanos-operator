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
	// TelemetryQuantiles is the configuration for the request telemetry quantiles.
	// +kubebuilder:validation:Optional
	TelemetryQuantiles *TelemetryQuantiles `json:"telemetryQuantiles,omitempty"`
	// WebConfig is the configuration for the Query UI and API web options.
	// +kubebuilder:validation:Optional
	WebConfig *WebConfig `json:"webConfig,omitempty"`
	// GRPCProxyStrategy is the strategy to use when proxying Series requests to leaf nodes.
	// +kubebuilder:validation:Enum=eager;lazy
	// +kubebuilder:default=eager
	GRPCProxyStrategy string `json:"grpcProxyStrategy,omitempty"`
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

// TelemetryQuantiles is the configuration for the request telemetry quantiles.
// Float usage is discouraged by controller-runtime, so we use string instead.
type TelemetryQuantiles struct {
	// Duration is the quantiles for exporting metrics about the request duration.
	// +kubebuilder:validation:Optional
	Duration []string `json:"duration,omitempty"`
	// Samples is the quantiles for exporting metrics about the samples count.
	// +kubebuilder:validation:Optional
	Samples []string `json:"samples,omitempty"`
	// Series is the quantiles for exporting metrics about the series count.
	// +kubebuilder:validation:Optional
	Series []string `json:"series,omitempty"`
}

// WebConfig is the configuration for the Query UI and API web options.
type WebConfig struct {
	// RoutePrefix is the prefix for API and UI endpoints.
	// This allows thanos UI to be served on a sub-path.
	// Defaults to the value of --web.external-prefix.
	// This option is analogous to --web.route-prefix of Prometheus.
	// +kubebuilder:validation:Optional
	RoutePrefix *string `json:"routePrefix,omitempty"`
	// ExternalPrefix is the static prefix for all HTML links and redirect URLs in the UI query web interface.
	// Actual endpoints are still served on / or the web.route-prefix.
	// This allows thanos UI to be served behind a reverse proxy that strips a URL sub-path.
	// +kubebuilder:validation:Optional
	ExternalPrefix *string `json:"externalPrefix,omitempty"`
	// PrefixHeader is the name of HTTP request header used for dynamic prefixing of UI links and redirects.
	// This option is ignored if web.external-prefix argument is set.
	// Security risk: enable this option only if a reverse proxy in front of thanos is resetting the header.
	// This allows thanos UI to be served on a sub-path.
	// +kubebuilder:validation:Optional
	PrefixHeader *string `json:"prefixHeader,omitempty"`
	// DisableCORS is the flag to disable CORS headers to be set by Thanos.
	// By default Thanos sets CORS headers to be allowed by all.
	// +kubebuilder:default=false
	DisableCORS *bool `json:"disableCORS,omitempty"`
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
