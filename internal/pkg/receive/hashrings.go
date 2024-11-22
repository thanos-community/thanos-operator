package receive

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"slices"
	"sort"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/receive"

	discoveryv1 "k8s.io/api/discovery/v1"
)

const GRPCPort = receive.GRPCPort

// TenantMatcher represents the type of tenant matching to use.
type TenantMatcher string

const (
	// TenantMatcherTypeExact matches tenants exactly. This is also the default one.
	TenantMatcherTypeExact TenantMatcher = "exact"
	// TenantMatcherGlob matches tenants using glob patterns.
	TenantMatcherGlob TenantMatcher = "glob"
)

// HashringAlgorithm represents the hashing algorithm to use.
type HashringAlgorithm string

const (
	// AlgorithmKetama is the ketama hashing algorithm.
	AlgorithmKetama HashringAlgorithm = "ketama"
	// AlgorithmHashmod is the hashmod hashing algorithm.
	AlgorithmHashmod HashringAlgorithm = "hashmod"
)

// Endpoint represents a single logical member of a hashring.
type Endpoint struct {
	// Address is the address of the endpoint.
	Address string `json:"address"`
	// AZ is the availability zone of the endpoint.
	AZ string `json:"az"`
}

type HashringMeta struct {
	DesiredReplicas int
	Config          HashringConfig
}

// HashringState represents the desired state of a hashring.
// It is a map of hashring name to hashring metadata.
type HashringState map[string]HashringMeta

// HashringConfig represents the configuration for a hashring a receiver node knows about.
type HashringConfig struct {
	// Name is the name of the hashring.
	Name string `json:"hashring,omitempty"`
	// Tenants is a list of tenants that match on this hashring.
	Tenants []string `json:"tenants,omitempty"`
	// TenantMatcherType is the type of tenant matching to use.
	TenantMatcherType TenantMatcher `json:"tenant_matcher_type,omitempty"`
	// Endpoints is a list of endpoints that are part of this hashring.
	Endpoints []Endpoint `json:"endpoints"`
	// Algorithm is the hashing algorithm to use.
	Algorithm HashringAlgorithm `json:"algorithm,omitempty"`
	// ExternalLabels are the external labels to use for this hashring.
	ExternalLabels labels.Labels `json:"external_labels,omitempty"`
}

// MapToExternalLabels converts a map to external labels.
func MapToExternalLabels(m map[string]string) labels.Labels {
	builder := labels.NewScratchBuilder(len(m))
	for k, v := range m {
		builder.Add(k, v)
	}

	return builder.Labels()
}

// Hashrings is a list of hashrings.
type Hashrings []HashringConfig

// EndpointFilter is a function that filters endpoints.
type EndpointFilter func() func(ep discoveryv1.EndpointSlice) []discoveryv1.Endpoint

// FilterEndpointByOwnerRef returns an EndpointFilter that filters EndpointSlices by owner reference.
func FilterEndpointByOwnerRef(expectOwner string) EndpointFilter {
	return func() func(ep discoveryv1.EndpointSlice) []discoveryv1.Endpoint {
		return func(ep discoveryv1.EndpointSlice) []discoveryv1.Endpoint {
			for _, owner := range ep.OwnerReferences {
				if owner.Name == expectOwner {
					return ep.Endpoints
				}
			}
			return nil
		}
	}
}

// FilterEndpointReady returns an EndpointFilter that filters EndpointSlices by ready condition.
func FilterEndpointReady() EndpointFilter {
	return func() func(eps discoveryv1.EndpointSlice) []discoveryv1.Endpoint {
		return func(eps discoveryv1.EndpointSlice) []discoveryv1.Endpoint {
			var readyEndpoints []discoveryv1.Endpoint
			for _, ep := range eps.Endpoints {
				if ep.Conditions.Ready != nil && *ep.Conditions.Ready {
					readyEndpoints = append(readyEndpoints, ep)
				}
			}
			return readyEndpoints
		}
	}
}

// EndpointConverter is a function that converts an EndpointSlice to an Endpoint.
type EndpointConverter func(eps discoveryv1.EndpointSlice, ep discoveryv1.Endpoint) Endpoint

// DefaultEndpointConverter is the default EndpointConverter that converts an EndpointSlice to an Endpoint.
// It uses the service name and namespace from the EndpointSlice to construct the address.
func DefaultEndpointConverter(eps discoveryv1.EndpointSlice, ep discoveryv1.Endpoint) Endpoint {
	svcName := eps.Labels[discoveryv1.LabelServiceName]
	ns := eps.GetNamespace()
	return Endpoint{
		Address: fmt.Sprintf("%s.%s.%s.svc.cluster.local:%d", *ep.Hostname, svcName, ns, GRPCPort),
	}
}

// EndpointSliceListToEndpoints converts a list of EndpointSlices to a list of Endpoints.
// It uses the provided EndpointConverter to convert each EndpointSlice to an Endpoint.
// It also applies the provided EndpointFilters to filter the EndpointSlices.
func EndpointSliceListToEndpoints(converter EndpointConverter, eps discoveryv1.EndpointSliceList, filters ...EndpointFilter) []Endpoint {
	var endpoints []Endpoint
	for _, epSlice := range eps.Items {
		for _, filter := range filters {
			fn := filter()
			epSlice.Endpoints = fn(epSlice)
		}

		svcEndpoints := epSlice.Endpoints
		for _, ep := range svcEndpoints {
			endpoints = append(endpoints, converter(epSlice, ep))
		}
	}
	sort.Slice(endpoints, func(i, j int) bool {
		return endpoints[i].Address < endpoints[j].Address
	})
	return slices.Compact(endpoints)
}

// DynamicMerge merges the previous state of hashrings with the desired state.
// It ensures that the hashrings are in the desired state and that the replication factor is met.
// If the previous state is empty, it will only add hashrings that have all members ready.
// It allows for a single missing member to account for voluntary disruptions.
func DynamicMerge(previousState Hashrings, desiredState HashringState, replicationFactor int) Hashrings {
	var mergedState Hashrings
	if isEmptyHashring(previousState) {
		return handleUnseenHashrings(desiredState)
	}
	for k, v := range desiredState {
		// we first check that the hashring can meet the desired replication factor
		// secondly, we allow to tolerate a single missing member. this allows us to account for
		// voluntary disruptions to the hashring.
		// todo - allow for more than one missing member based on input from PDB settings etc
		if len(v.Config.Endpoints) >= replicationFactor && len(v.Config.Endpoints) >= v.DesiredReplicas-1 {
			mergedState = append(mergedState, metaToHashring(k, v))
			continue
		}
		// otherwise we look for previous state and merge if it exists
		// this means that if the hashring is having issues, we don't interfere with it
		// since doing so could cause further disruptions and frequent reshuffling
		for _, hr := range previousState {
			if hr.Name == k {
				mergedState = append(mergedState, hr)
			}
		}
	}
	sort.Slice(mergedState, func(i, j int) bool {
		return mergedState[i].Name < mergedState[j].Name
	})

	return mergedState
}

func handleUnseenHashrings(desiredState HashringState) Hashrings {
	var hashrings Hashrings
	for k, v := range desiredState {
		// we don't add anything until all members become ready initially
		if len(v.Config.Endpoints) >= v.DesiredReplicas {
			hashrings = append(hashrings, metaToHashring(k, v))
		}
	}
	return hashrings
}

func metaToHashring(key string, value HashringMeta) HashringConfig {
	return HashringConfig{
		Name:              key,
		Tenants:           value.Config.Tenants,
		TenantMatcherType: value.Config.TenantMatcherType,
		Endpoints:         value.Config.Endpoints,
		Algorithm:         value.Config.Algorithm,
		ExternalLabels:    value.Config.ExternalLabels,
	}
}

func isEmptyHashring(hashrings Hashrings) bool {
	if len(hashrings) == 0 || len(hashrings) == 1 && len(hashrings[0].Endpoints) == 0 {
		return true
	}

	return false
}

// UnmarshalJSON unmarshal the endpoint from JSON.
func (e *Endpoint) UnmarshalJSON(data []byte) error {
	// First try to unmarshal as a string.
	err := json.Unmarshal(data, &e.Address)
	if err == nil {
		return nil
	}

	// If that fails, try to unmarshal as an endpoint object.
	type endpointAlias Endpoint
	var configEndpoint endpointAlias
	err = json.Unmarshal(data, &configEndpoint)
	if err == nil {
		e.Address = configEndpoint.Address
		e.AZ = configEndpoint.AZ
	}
	return err
}

// HashAsMetricValue hashes the given data and returns a float64 value.
func HashAsMetricValue(data []byte) float64 {
	sum := md5.Sum(data)
	smallSum := sum[0:6]
	var bytes = make([]byte, 8)
	copy(bytes, smallSum)
	return float64(binary.LittleEndian.Uint64(bytes))
}
