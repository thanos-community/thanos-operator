package query

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"

	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
)

//nolint:tagliatelle // Thanos configuration uses snake_case.
func TestTLSEndpointDiscovery(t *testing.T) {
	types := []manifests.EndpointType{manifests.RegularLabel, manifests.StrictLabel, manifests.GroupLabel, manifests.GroupStrictLabel}
	endpoints := make([]Endpoint, len(types))
	for i, kind := range types {
		endpoints[i] = Endpoint{ServiceName: "store", Namespace: "metrics", Port: 10901, Type: kind, Address: "10.0.0.1:10901"}
	}
	var cfg struct {
		DefaultClientConfig struct {
			TLSConfig struct {
				Enabled bool   `json:"enabled"`
				CAFile  string `json:"ca_file"`
			} `json:"tls_config"`
		} `json:"default_client_config"`
		Endpoints []struct {
			Address       string
			Strict, Group bool
			ClientConfig  struct {
				ServerName string `json:"server_name"`
			} `json:"client_config"`
		}
	}
	require.NoError(t, json.Unmarshal([]byte(tlsEndpointConfig(endpoints)), &cfg))
	require.True(t, cfg.DefaultClientConfig.TLSConfig.Enabled)
	require.Equal(t, manifests.TLSCAFile, cfg.DefaultClientConfig.TLSConfig.CAFile)
	for i, ep := range cfg.Endpoints {
		require.Equal(t, i == 1 || i == 3, ep.Strict)
		require.Equal(t, i >= 2, ep.Group)
		require.Equal(t, "store.metrics.svc", ep.ClientConfig.ServerName)
		if ep.Strict {
			require.Equal(t, "store.metrics.svc:10901", ep.Address)
		} else if i == 0 {
			require.Equal(t, "10.0.0.1:10901", ep.Address)
		} else {
			require.Equal(t, "dnssrv+_grpc._tcp.store.metrics.svc", ep.Address)
		}
	}
}

func TestTLSMembershipChangesOnlyTheEndpointConfig(t *testing.T) {
	flags := featuregate.Flag{featuregate.TLS}
	opts := Options{Options: manifests.Options{Owner: "query", Namespace: "metrics", Config: flags.ToFeatureGate()}}
	before := NewQueryDeployment(opts)
	opts.Endpoints = []Endpoint{{ServiceName: "store", Namespace: "metrics", Port: 10901, Type: manifests.RegularLabel, Address: "10.0.0.1:10901"}}
	require.Equal(t, before.Spec.Template, NewQueryDeployment(opts).Spec.Template, "endpoint changes must not restart Query")
	for _, obj := range opts.Build() {
		if cm, ok := obj.(*corev1.ConfigMap); ok {
			require.Contains(t, cm.Data["endpoints.yaml"], "10.0.0.1:10901")
			return
		}
	}
	t.Fatal("missing endpoint ConfigMap")
}
