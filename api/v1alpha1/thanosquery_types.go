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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:openapi-gen=true
// +kubebuilder:resource:categories="thanos-operator",shortName="query"
// +kubebuilder:subresource:status

// ThanosQuerySpec defines the desired state of Thanos Query.
// +k8s:openapi-gen=true
type ThanosQuerySpec struct {
	CommonThanosFields `json:",inline"`

	// The external Thanos Query URL that would be set in all alerts 'Source' field.
	// +optional
	AlertQueryURL *string `json:"alertQueryURL,omitempty" opt:"alert.query-url"`

	// Addresses of statically configured Thanos API servers.
	// The scheme may be prefixed with 'dns+' or 'dnssrv+' to detect Thanos API
	// servers through respective DNS lookups.
	// Only to be used for out-of-cluster connections.
	// +optional
	Endpoint []string `json:"endpoint" opt:"endpoint"`
	// Experimental: DNS name of statically configured Thanos API server groups.
	// Targets resolved from the DNS name will be queried in a round-robin, instead of a fanout manner.
	// This flag should be used when connecting a Thanos Query to HA groups of Thanos components.
	// Only to be used for out-of-cluster connections.
	// +optional
	EndpointGroup []string `json:"endpointGroup" opt:"endpoint-group"`
	// Addresses of only statically configured Thanos API servers that are always used,
	// even if the health check fails. Useful if you have a caching layer on top.
	// Only to be used for out-of-cluster connections.
	// +optional
	EndpointStrict []string `json:"endpointStrict" opt:"endpoint-strict"`
	// Experimental: Addresses of only statically configured Thanos API servers groups that are always used,
	// even if the health check fails. Useful if you have a caching layer on top.
	// Only to be used for out-of-cluster connections.
	// +optional
	EndpointGroupStrict []string `json:"endpointGroupStrict" opt:"endpoint-group-strict"`

	// Default behavior of the Operator is to add new Thanos API servers to an SD file.
	// Setting this to true, ensures that the operator will instead discover Thanos API
	// servers and configure them on querier as --endpoint flags and redeploy.
	// +kubebuilder:default:=false
	UseFlagForAutoSD bool `json:"useFileForAutoSD"`

	// TODO(saswatamcode): Figure out a neat way to handle this.
	// GRPCClientsServerName    string `json:"grpcClientServer-name"`
	// GRPCClientsTLSCA         string `json:"grpcClientTLSCA"`
	// GRPCClientsTLSCert       string `json:"grpcClientTLSCert"`
	// GRPCClientsTLSKey        string `json:"grpcClientTLSKey"`
	// GRPCClientsTLSSecure     bool   `json:"grpcClientTLSSecure"`
	// GRPCClientsTLSSkipVerify bool   `json:"grpcClientTLSSkipVerify"`

	// GRPCClientsCompression string `json:"grpcCompression"`

	// GRPCAddress           string        `json:"grpcAddress"`
	// GRPCGracePeriod       time.Duration `json:"grpcGracePeriod"`
	// GRPCMMaxConnectionAge time.Duration `json:"grpcServerMaxConnectionAge"`
	// GRPCServerTLSCert     string        `json:"grpcServerTLSCert"`
	// GRPCServerTLSClientCA string        `json:"grpcServerTLSClientCA"`
	// GRPCServerTLSKey      string        `json:"grpcServerTLSKey"`

	// HTTPAddress     string        `json:"httpAddress"`
	// HTTPGracePeriod time.Duration `json:"httpGracePeriod"`
	// HTTPConfig      string        `json:"httpConfig"`

	// Directory to log currently active queries in the queries.active file.
	// +optional
	ActiveQueryPath *string `json:"activeQueryPath,omitempty" opt:"query.active-query-path"`
	// Enable automatic adjustment (step / 5) to what source of data should be used in store gateways if no max_source_resolution param is specified.
	// +kubebuilder:default:=true
	AutoDownsampling bool `json:"autoDownsampling,omitempty" opt:"query.auto-downsampling"`
	// Default evaluation interval for sub queries.
	// +kubebuilder:default:="1m"
	// +optional
	DefaultEvaluationInterval *time.Duration `json:"defaultEvaluationInterval,omitempty" opt:"query.default-evaluation-interval"`
	// Set default step for range queries. Default step is only used when step is not set in UI.
	// In such cases, Thanos UI will use default step to calculate resolution (resolution = max(rangeSeconds / 250, defaultStep)).
	// This will not work from Grafana, but Grafana has __step variable which can be used
	// +kubebuilder:default:="1s"
	// +optional
	DefaultStep *time.Duration `json:"defaultStep,omitempty" opt:"query.default-step"`
	// The maximum lookback duration for retrieving metrics during expression evaluations.
	// PromQL always evaluates the query for the certain timestamp (query range timestamps are deduced by step).
	// Since scrape intervals might be different, PromQL looks back for given amount of time to get latest sample.
	// If it exceeds the maximum lookback delta it assumes series is stale and returns none (a gap).
	// This is why lookback delta should be set to at least 2 times of the slowest scrape interval. If unset it will use the promql default of 5m.
	// +optional
	LookbackDelta *time.Duration `json:"lookbackDelta,omitempty" opt:"query.lookback-delta"`
	// Allow for larger lookback duration for queries based on resolution.
	DynamicLookbackDelta bool `json:"dynamicLookbackDelta,omitempty" opt:"query.dynamic-lookback-delta"`
	// Maximum number of queries processed concurrently by query node.
	// +kubebuilder:default:="20"
	MaxConcurrent int `json:"maxConcurrent,omitempty" opt:"query.max-concurrent"`
	// Maximum number of select requests made concurrently per a query.
	// +kubebuilder:default:="4"
	MaxConcurrentSelect int `json:"maxConcurrentSelect,omitempty" opt:"query.max-concurrent-select"`
	// The default metadata time range duration for retrieving labels through Labels and Series API
	// when the range parameters are not specified. The zero value means range covers the time since the beginning.
	// +kubebuilder:default:="0s"
	MetadataDefaultTimeRange time.Duration `json:"metadataDefaultTimeRange,omitempty" opt:"query.metadata.default-time-range"`
	// Enable partial response for queries if no partial_response param is specified. --no-query.partial-response for disabling.
	// +kubebuilder:default:=true
	PartialResponse bool `json:"partialResponse,omitempty" opt:"query.partial-response"`
	// Default PromQL engine to use.
	// +kubebuilder:validation:Enum=thanos;prometheus
	// +kubebuilder:default:="thanos"
	PromQLEngine string `json:"promQLEngine,omitempty" opt:"query.promql-engine"`
	// Labels to treat as a replica indicator along which data is deduplicated.
	// Still you will be able to query without deduplication using 'dedup=false' parameter.
	// Data includes time series, recording rules, and alerting rules.
	ReplicaLabels []string `json:"replicaLabels,omitempty" opt:"query.replica-label"`

	// QueryTelemetryRequestDurationSecondsQuantiles []float64     `opt:"query.telemetry.request-duration-seconds-quantiles"`
	// QueryTelemetryRequestSamplesQuantiles         []float64     `opt:"query.telemetry.request-samples-quantiles"`
	// QueryTelemetryRequestSeriesSecondsQuantiles   []float64     `opt:"query.telemetry.request-series-seconds-quantiles"`
	// QueryTenantCertificateField                   string        `opt:"query.tenant-certificate-field"`
	// QueryTenantHeader                             string        `opt:"query.tenant-header"`
	// QueryTimeout                                  time.Duration `opt:"query.timeout"`

	// TODO(saswatamcode): Figure out a neat way to handle this.
	// RequestLoggingConfig                          *reqlogging.RequestConfig      `json:"request.logging-config"`
	// RequestLoggingConfigFile                      containeropts.ContainerUpdater `json:"request.logging-config-file"`

	// SelectorLabel             []string      `json:"selector-label"`
	// StoreLimitsRequestSamples int           `json:"store.limits.request-samples"`
	// StoreLimitsRequestSeries  int           `json:"store.limits.request-series"`
	// StoreResponseTimeout      time.Duration `json:"store.response-timeout"`
	// StoreSDDNSResolver        string        `json:"store.sd-dns-resolver"`
	// StoreSDDNSInterval        time.Duration `json:"store.sd-dns-interval"`
	// StoreSDFiles              []string      `json:"store.sd-files"`
	// StoreSDInterval           time.Duration `json:"store.sd-interval"`
	// StoreUnhealthyTimeout     time.Duration `json:"store.unhealthy-timeout"`

	// TODO(saswatamcode): Figure out a neat way to handle this.
	// TracingConfig                                 *trclient.TracingConfig        `json:"tracing.config"`
	// TracingConfigFile                             containeropts.ContainerUpdater `json:"tracing.config-file"`

	// SelectorRelabelConfig string `json:"selector.relabel-config"`

	// TODO(saswatamcode): Figure out a neat way to handle this.
	// WebDisableCORS    bool   `json:"web.disable-cors"`
	// WebExternalPrefix string `json:"web.external-prefix"`
	// WebPrefixHeader   string `json:"web.prefix-header"`
	// WebRoutePrefix    string `json:"web.route-prefix"`
}

// ThanosQueryStatus defines the observed state of ThanosQuery
type ThanosQueryStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
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
