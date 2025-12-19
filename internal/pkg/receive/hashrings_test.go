package receive

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/assert"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

const (
	clusterDomain = "cluster.local"
)

func TestDefaultEndpointConverter(t *testing.T) {
	eps := discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Labels: map[string]string{
				discoveryv1.LabelServiceName: "test-service",
			},
		},
		Endpoints: []discoveryv1.Endpoint{
			{
				Hostname: ptr.To("test-host"),
				Conditions: discoveryv1.EndpointConditions{
					Ready: ptr.To(true),
				},
			},
		},
	}

	ep := discoveryv1.Endpoint{
		Hostname: ptr.To("test-host"),
		Conditions: discoveryv1.EndpointConditions{
			Ready: ptr.To(true),
		},
	}

	expected := Endpoint{
		Address: fmt.Sprintf("test-host.test-service.default.svc.%s:10901", clusterDomain),
	}

	result := DefaultEndpointConverter(eps, ep, clusterDomain)
	if result != expected {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestCapnProtoEndpointConverter(t *testing.T) {
	eps := discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Labels: map[string]string{
				discoveryv1.LabelServiceName: "test-service",
			},
		},
		Endpoints: []discoveryv1.Endpoint{
			{
				Hostname: ptr.To("test-host"),
				Conditions: discoveryv1.EndpointConditions{
					Ready: ptr.To(true),
				},
			},
		},
	}

	ep := discoveryv1.Endpoint{
		Hostname: ptr.To("test-host"),
		Conditions: discoveryv1.EndpointConditions{
			Ready: ptr.To(true),
		},
	}

	expected := Endpoint{
		Address:          fmt.Sprintf("test-host.test-service.default.svc.%s:10901", clusterDomain),
		CapnProtoAddress: fmt.Sprintf("test-host.test-service.default.svc.%s:19391", clusterDomain),
	}

	result := CapnProtoEndpointConverter(eps, ep, clusterDomain)
	if result != expected {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestFilterEndpointReady(t *testing.T) {
	eps := discoveryv1.EndpointSlice{
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"test-host"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: ptr.To(true),
				},
			},
			{
				Conditions: discoveryv1.EndpointConditions{
					Ready: ptr.To(false),
				},
			},
		},
	}

	filter := FilterEndpointReady()
	result := filter()(eps)
	if len(result) != 1 {
		t.Errorf("expected 1 endpoint, got %d", len(result))
	}
	if !*result[0].Conditions.Ready {
		t.Errorf("expected endpoint to be ready")
	}
	if result[0].Addresses[0] != "test-host" {
		t.Errorf("expected address to be test-host, got %s", result[0].Addresses[0])
	}
}

func TestFilterEndpointByOwnerRef(t *testing.T) {
	tests := []struct {
		name          string
		expectOwner   string
		endpointSlice discoveryv1.EndpointSlice
		expectedCount int
		expectedHost  string
	}{
		{
			name:        "WithMatchingOwner",
			expectOwner: "expected-owner",
			endpointSlice: discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{Name: "expected-owner"},
					},
				},
				Endpoints: []discoveryv1.Endpoint{
					{Hostname: ptr.To("test-host")},
				},
			},
			expectedCount: 1,
			expectedHost:  "test-host",
		},
		{
			name:        "WithNonMatchingOwner",
			expectOwner: "expected-owner",
			endpointSlice: discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{Name: "unexpected-owner"},
					},
				},
				Endpoints: []discoveryv1.Endpoint{
					{Hostname: ptr.To("test-host")},
				},
			},
			expectedCount: 0,
		},
		{
			name:        "WithNoOwnerReferences",
			expectOwner: "expected-owner",
			endpointSlice: discoveryv1.EndpointSlice{
				Endpoints: []discoveryv1.Endpoint{
					{Hostname: ptr.To("test-host")},
				},
			},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := FilterEndpointByOwnerRef(tt.expectOwner)
			result := filter()(tt.endpointSlice)
			if len(result) != tt.expectedCount {
				t.Errorf("expected %d endpoints, got %d", tt.expectedCount, len(result))
			}
			if tt.expectedCount > 0 && *result[0].Hostname != tt.expectedHost {
				t.Errorf("expected hostname to be '%s', got '%s'", tt.expectedHost, *result[0].Hostname)
			}
		})
	}
}

func TestEndpointSliceListToEndpoints(t *testing.T) {
	tests := []struct {
		name      string
		eps       discoveryv1.EndpointSliceList
		converter EndpointConverter
		filters   []EndpointFilter
		expected  []Endpoint
	}{
		{
			name:      "EmptyEndpointSliceList",
			eps:       discoveryv1.EndpointSliceList{},
			converter: DefaultEndpointConverter,
			expected:  []Endpoint{},
		},
		{
			name: "SingleEndpoint",
			eps: discoveryv1.EndpointSliceList{
				Items: []discoveryv1.EndpointSlice{
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "default",
							Labels: map[string]string{
								discoveryv1.LabelServiceName: "test-service",
							},
						},
						Endpoints: []discoveryv1.Endpoint{
							{
								Hostname: ptr.To("test-host"),
								Conditions: discoveryv1.EndpointConditions{
									Ready: ptr.To(true),
								},
							},
						},
					},
				},
			},
			converter: DefaultEndpointConverter,
			expected: []Endpoint{
				{
					Address: fmt.Sprintf("test-host.test-service.default.svc.%s:10901", clusterDomain),
				},
			},
		},
		{
			name: "WithFilters",
			eps: discoveryv1.EndpointSliceList{
				Items: []discoveryv1.EndpointSlice{
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "default",
							Labels: map[string]string{
								discoveryv1.LabelServiceName: "test-service",
							},
						},
						Endpoints: []discoveryv1.Endpoint{
							{
								Hostname: ptr.To("test-host"),
								Conditions: discoveryv1.EndpointConditions{
									Ready: ptr.To(true),
								},
							},
							{
								Hostname: ptr.To("test-host-2"),
								Conditions: discoveryv1.EndpointConditions{
									Ready: ptr.To(false),
								},
							},
						},
					},
				},
			},
			converter: DefaultEndpointConverter,
			filters:   []EndpointFilter{FilterEndpointReady()},
			expected: []Endpoint{
				{
					Address: fmt.Sprintf("test-host.test-service.default.svc.%s:10901", clusterDomain),
				},
			},
		},
		{
			name: "WithCapnProtoEndpointConverter",
			eps: discoveryv1.EndpointSliceList{
				Items: []discoveryv1.EndpointSlice{
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "default",
							Labels: map[string]string{
								discoveryv1.LabelServiceName: "test-service",
							},
						},
						Endpoints: []discoveryv1.Endpoint{
							{
								Hostname: ptr.To("test-host"),
								Conditions: discoveryv1.EndpointConditions{
									Ready: ptr.To(true),
								},
							},
							{
								Hostname: ptr.To("test-host-2"),
								Conditions: discoveryv1.EndpointConditions{
									Ready: ptr.To(false),
								},
							},
						},
					},
				},
			},
			converter: CapnProtoEndpointConverter,
			filters:   []EndpointFilter{FilterEndpointReady()},
			expected: []Endpoint{
				{
					Address:          fmt.Sprintf("test-host.test-service.default.svc.%s:10901", clusterDomain),
					CapnProtoAddress: fmt.Sprintf("test-host.test-service.default.svc.%s:19391", clusterDomain),
				},
			},
		},
		{
			name: "MultipleFilters",
			eps: discoveryv1.EndpointSliceList{
				Items: []discoveryv1.EndpointSlice{
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "default",
							Labels: map[string]string{
								discoveryv1.LabelServiceName: "test-service",
							},
							OwnerReferences: []metav1.OwnerReference{
								{Name: "expected-owner"},
							},
						},
						Endpoints: []discoveryv1.Endpoint{
							{
								Hostname: ptr.To("test-host"),
								Conditions: discoveryv1.EndpointConditions{
									Ready: ptr.To(true),
								},
							},
							{
								Hostname: ptr.To("test-host-2"),
								Conditions: discoveryv1.EndpointConditions{
									Ready: ptr.To(false),
								},
							},
						},
					},
				},
			},
			converter: DefaultEndpointConverter,
			filters:   []EndpointFilter{FilterEndpointReady(), FilterEndpointByOwnerRef("expected-owner")},
			expected: []Endpoint{
				{
					Address: fmt.Sprintf("test-host.test-service.default.svc.%s:10901", clusterDomain),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EndpointSliceListToEndpoints(tt.converter, tt.eps, clusterDomain, tt.filters...)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d endpoints, got %d", len(tt.expected), len(result))
			}
			for i, ep := range result {
				if ep != tt.expected[i] {
					t.Errorf("expected %v, got %v", tt.expected[i], ep)
				}
			}
		})
	}
}

const hashringName = "hashring1"

func TestDynamicMergeEmptyPreviousState(t *testing.T) {
	previousState := Hashrings{}
	desiredState := HashringState{
		hashringName: {
			DesiredReplicas: 3,
			Config: HashringConfig{
				Endpoints: []Endpoint{
					{Address: "endpoint1"},
					{Address: "endpoint2"},
					{Address: "endpoint3"},
				},
			},
		},
	}
	replicationFactor := 3

	result := DynamicMerge(previousState, desiredState, replicationFactor)
	if len(result) != 1 {
		t.Errorf("expected 1 hashring, got %d", len(result))
	}
	if result[0].Name != hashringName {
		t.Errorf("expected hashring name 'hashring1', got '%s'", result[0].Name)
	}
}

func TestDynamicMergeReplicationFactorMet(t *testing.T) {
	previousState := Hashrings{
		{
			Name: hashringName,
			Endpoints: []Endpoint{
				{Address: "endpoint1"},
				{Address: "endpoint2"},
				{Address: "endpoint3"},
			},
		},
	}
	desiredState := HashringState{
		hashringName: {
			DesiredReplicas: 3,
			Config: HashringConfig{
				Endpoints: []Endpoint{
					{Address: "endpoint1"},
					{Address: "endpoint2"},
					{Address: "endpoint3"},
				},
			},
		},
	}
	replicationFactor := 3

	result := DynamicMerge(previousState, desiredState, replicationFactor)
	if len(result) != 1 {
		t.Errorf("expected 1 hashring, got %d", len(result))
	}
	if result[0].Name != hashringName {
		t.Errorf("expected hashring name 'hashring1', got '%s'", result[0].Name)
	}
}

func TestDynamicMergeShouldRestorePreviousState(t *testing.T) {
	previousState := Hashrings{
		{
			Name: hashringName,
			Endpoints: []Endpoint{
				{Address: "endpoint1"},
				{Address: "endpoint2"},
				{Address: "endpoint3"},
			},
		},
	}
	desiredState := HashringState{
		hashringName: {
			DesiredReplicas: 3,
			Config: HashringConfig{
				Endpoints: []Endpoint{
					{Address: "endpoint1"},
					{Address: "endpoint2"},
				},
			},
		},
	}
	replicationFactor := 3

	result := DynamicMerge(previousState, desiredState, replicationFactor)
	if len(result) != 1 {
		t.Errorf("expected 1 hashring, got %d", len(result))
	}
	if result[0].Name != hashringName {
		t.Errorf("expected hashring name 'hashring1', got '%s'", result[0].Name)
	}
	if !reflect.DeepEqual(result[0].Endpoints, previousState[0].Endpoints) {
		t.Errorf("expected endpoints to be %v, got %v", previousState[0].Endpoints, result[0].Endpoints)
	}
}

func TestDynamicMergeShouldAllowMissingMember(t *testing.T) {
	previousState := Hashrings{
		{
			Name: hashringName,
			Endpoints: []Endpoint{
				{Address: "endpoint1"},
				{Address: "endpoint2"},
				{Address: "endpoint3"},
				{Address: "endpoint4"},
			},
		},
	}
	desiredState := HashringState{
		hashringName: {
			DesiredReplicas: 4,
			Config: HashringConfig{
				Endpoints: []Endpoint{
					{Address: "endpoint1"},
					{Address: "endpoint2"},
					{Address: "endpoint3"},
				},
			},
		},
	}
	replicationFactor := 3

	result := DynamicMerge(previousState, desiredState, replicationFactor)
	if len(result) != 1 {
		t.Errorf("expected 1 hashring, got %d", len(result))
	}
	if result[0].Name != hashringName {
		t.Errorf("expected hashring name 'hashring1', got '%s'", result[0].Name)
	}
	if !reflect.DeepEqual(result[0].Endpoints, desiredState[hashringName].Config.Endpoints) {
		t.Errorf("expected endpoints to be %v, got %v", desiredState[hashringName].Config.Endpoints, result[0].Endpoints)
	}
}

func TestDynamicMergeDesiredStateNotMet(t *testing.T) {
	previousState := Hashrings{
		{
			Name: hashringName,
			Endpoints: []Endpoint{
				{Address: "endpoint1"},
			},
		},
	}
	desiredState := HashringState{
		hashringName: {
			DesiredReplicas: 3,
			Config: HashringConfig{
				Endpoints: []Endpoint{
					{Address: "endpoint1"},
				},
			},
		},
	}
	replicationFactor := 3

	result := DynamicMerge(previousState, desiredState, replicationFactor)
	if len(result) != 1 {
		t.Errorf("expected 1 hashring, got %d", len(result))
	}
	if result[0].Name != hashringName {
		t.Errorf("expected hashring name 'hashring1', got '%s'", result[0].Name)
	}
}

func TestMapToExternalLabels(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		expected labels.Labels
	}{
		{
			name:     "EmptyMap",
			input:    map[string]string{},
			expected: labels.Labels{},
		},
		{
			name: "SingleEntry",
			input: map[string]string{
				"key1": "value1",
			},
			expected: labels.FromMap(map[string]string{
				"key1": "value1",
			}),
		},
		{
			name: "MultipleEntries",
			input: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			expected: labels.FromMap(map[string]string{
				"key1": "value1",
				"key2": "value2",
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MapToExternalLabels(tt.input)
			if result.Len() != tt.expected.Len() {
				t.Errorf("expected %d labels, got %d", tt.expected.Len(), result.Len())
			}
			for name, value := range tt.expected.Map() {
				if result.Get(name) != value {
					t.Errorf("expected label %s to have value %s, got %s", name, value, result.Get(name))
				}

			}
		})
	}
}

func TestStaticMergeEmptyPreviousState(t *testing.T) {
	previousState := Hashrings{}
	desiredState := HashringState{
		hashringName: {
			DesiredReplicas: 3,
			Config: HashringConfig{
				Endpoints: []Endpoint{
					{Address: "endpoint1"},
					{Address: "endpoint2"},
					{Address: "endpoint3"},
				},
			},
		},
	}
	replicationFactor := 3

	result := StaticMerge(previousState, desiredState, replicationFactor)
	if len(result) != 1 {
		t.Errorf("expected 1 hashring, got %d", len(result))
	}
	if result[0].Name != hashringName {
		t.Errorf("expected hashring name 'hashring1', got '%s'", result[0].Name)
	}

	if len(result[0].Endpoints) != 3 {
		t.Errorf("expected 3 endpoints, got %d", len(result[0].Endpoints))
	}
}

func TestStaticMergeEmptyPreviousStateNotReady(t *testing.T) {
	previousState := Hashrings{}
	desiredState := HashringState{
		hashringName: {
			DesiredReplicas: 5,
			Config: HashringConfig{
				Endpoints: []Endpoint{
					{Address: "endpoint1"},
					{Address: "endpoint2"},
					{Address: "endpoint3"},
				},
			},
		},
	}
	replicationFactor := 3

	result := StaticMerge(previousState, desiredState, replicationFactor)
	if len(result) != 0 {
		t.Errorf("expected 0 hashring, got %d", len(result))
	}
}

func TestStaticMergeReplicationFactorMet(t *testing.T) {
	previousState := Hashrings{
		{
			Name: hashringName,
			Endpoints: []Endpoint{
				{Address: "endpoint1"},
				{Address: "endpoint2"},
				{Address: "endpoint3"},
			},
		},
	}
	desiredState := HashringState{
		hashringName: {
			DesiredReplicas: 3,
			Config: HashringConfig{
				Endpoints: []Endpoint{
					{Address: "endpoint1"},
					{Address: "endpoint2"},
					{Address: "endpoint3"},
				},
			},
		},
	}
	replicationFactor := 3

	result := StaticMerge(previousState, desiredState, replicationFactor)
	if len(result) != 1 {
		t.Errorf("expected 1 hashring, got %d", len(result))
	}
	if result[0].Name != hashringName {
		t.Errorf("expected hashring name 'hashring1', got '%s'", result[0].Name)
	}

	if len(result[0].Endpoints) != 3 {
		t.Errorf("expected 3 endpoints, got %d", len(result[0].Endpoints))
	}
}

func TestStaticMergeReplicationFactorMetScaleUpNotReady(t *testing.T) {
	previousState := Hashrings{
		{
			Name: hashringName,
			Endpoints: []Endpoint{
				{Address: "endpoint1"},
				{Address: "endpoint2"},
				{Address: "endpoint3"},
			},
		},
	}
	desiredState := HashringState{
		hashringName: {
			DesiredReplicas: 5,
			Config: HashringConfig{
				Endpoints: []Endpoint{
					{Address: "endpoint1"},
					{Address: "endpoint2"},
					{Address: "endpoint3"},
				},
			},
		},
	}
	replicationFactor := 3

	result := StaticMerge(previousState, desiredState, replicationFactor)
	if len(result) != 1 {
		t.Errorf("expected 1 hashring, got %d", len(result))
	}
	if result[0].Name != hashringName {
		t.Errorf("expected hashring name 'hashring1', got '%s'", result[0].Name)
	}

	if len(result[0].Endpoints) != 3 {
		t.Errorf("expected 3 endpoints, got %d", len(result[0].Endpoints))
	}
}

func TestStaticMergeReplicationFactorMetScaleDown(t *testing.T) {
	previousState := Hashrings{
		{
			Name: hashringName,
			Endpoints: []Endpoint{
				{Address: "endpoint-1"},
				{Address: "endpoint-2"},
				{Address: "endpoint-3"},
				{Address: "endpoint-4"},
				{Address: "endpoint-5"},
			},
		},
	}
	desiredState := HashringState{
		hashringName: {
			DesiredReplicas: 3,
			Config: HashringConfig{
				Endpoints: []Endpoint{
					{Address: "endpoint-5"},
					{Address: "endpoint-4"},
					{Address: "endpoint-1"},
					{Address: "endpoint-2"},
					{Address: "endpoint-3"},
				},
			},
		},
	}
	replicationFactor := 3

	result := StaticMerge(previousState, desiredState, replicationFactor)
	if len(result) != 1 {
		t.Errorf("expected 1 hashring, got %d", len(result))
	}
	if result[0].Name != hashringName {
		t.Errorf("expected hashring name 'hashring1', got '%s'", result[0].Name)
	}

	if len(result[0].Endpoints) != 3 {
		t.Errorf("expected 3 endpoints, got %d", len(result[0].Endpoints))
	}
}

func TestStaticMergeShouldRestorePreviousState(t *testing.T) {
	previousState := Hashrings{
		{
			Name: hashringName,
			Endpoints: []Endpoint{
				{Address: "endpoint1"},
				{Address: "endpoint2"},
				{Address: "endpoint3"},
			},
		},
	}
	desiredState := HashringState{
		hashringName: {
			DesiredReplicas: 3,
			Config: HashringConfig{
				Endpoints: []Endpoint{
					{Address: "endpoint1"},
					{Address: "endpoint2"},
				},
			},
		},
	}
	replicationFactor := 3

	result := StaticMerge(previousState, desiredState, replicationFactor)
	if len(result) != 1 {
		t.Errorf("expected 1 hashring, got %d", len(result))
	}
	if result[0].Name != hashringName {
		t.Errorf("expected hashring name 'hashring1', got '%s'", result[0].Name)
	}
	if !reflect.DeepEqual(result[0].Endpoints, previousState[0].Endpoints) {
		t.Errorf("expected endpoints to be %v, got %v", previousState[0].Endpoints, result[0].Endpoints)
	}
}

func TestStaticMergeDesiredStateNotMet(t *testing.T) {
	previousState := Hashrings{
		{
			Name: hashringName,
			Endpoints: []Endpoint{
				{Address: "endpoint1"},
			},
		},
	}
	desiredState := HashringState{
		hashringName: {
			DesiredReplicas: 3,
			Config: HashringConfig{
				Endpoints: []Endpoint{
					{Address: "endpoint1"},
				},
			},
		},
	}
	replicationFactor := 3

	result := StaticMerge(previousState, desiredState, replicationFactor)
	if len(result) != 1 {
		t.Errorf("expected 1 hashring, got %d", len(result))
	}
	if result[0].Name != hashringName {
		t.Errorf("expected hashring name 'hashring1', got '%s'", result[0].Name)
	}
}

func TestHashAsMetricValue(t *testing.T) {
	tests := []struct {
		data     []byte
		expected float64
	}{
		{
			data:     []byte(`foo`),
			expected: float64(2.80823948650028e+14),
		},
		{
			data:     []byte(`bar`),
			expected: float64(1.82360791047932e+14),
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.data), func(t *testing.T) {
			assert.Equal(t, tt.expected, HashAsMetricValue(tt.data))
		})
	}
}
