package query

import (
	"fmt"
	"sort"

	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

const (
	endpointSDConfigKey      = "endpoints.yaml"
	endpointSDVolumeName     = "endpoint-sd-config"
	endpointSDMountPath      = "/etc/thanos/endpoint-sd"
	endpointSDReloadInterval = "30s"
)

type endpointSDConfig struct {
	Endpoints []endpointSDEntry `json:"endpoints"`
}

type endpointSDEntry struct {
	Address string `json:"address"`
	Group   bool   `json:"group,omitempty"`
	Strict  bool   `json:"strict,omitempty"`
}

func (e Endpoint) sdEntry() endpointSDEntry {
	switch e.Type {
	case manifests.RegularLabel:
		return endpointSDEntry{
			Address: fmt.Sprintf("dnssrv+_grpc._tcp.%s.%s.svc", e.ServiceName, e.Namespace),
		}
	case manifests.GroupLabel:
		return endpointSDEntry{
			Address: fmt.Sprintf("dnssrv+_grpc._tcp.%s.%s.svc", e.ServiceName, e.Namespace),
			Group:   true,
		}
	case manifests.StrictLabel:
		// Static host:port. Thanos rejects a dynamic address in strict mode.
		return endpointSDEntry{
			Address: fmt.Sprintf("%s.%s.svc:%d", e.ServiceName, e.Namespace, e.Port),
			Strict:  true,
		}
	case manifests.GroupStrictLabel:
		// Static host:port. Thanos rejects a dynamic address in strict mode.
		return endpointSDEntry{
			Address: fmt.Sprintf("%s.%s.svc:%d", e.ServiceName, e.Namespace, e.Port),
			Group:   true,
			Strict:  true,
		}
	default:
		panic("unknown endpoint type")
	}
}

func renderEndpointSDConfig(endpoints []Endpoint) string {
	sorted := append([]Endpoint(nil), endpoints...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].ServiceName != sorted[j].ServiceName {
			return sorted[i].ServiceName < sorted[j].ServiceName
		}
		if sorted[i].Namespace != sorted[j].Namespace {
			return sorted[i].Namespace < sorted[j].Namespace
		}
		if sorted[i].Type != sorted[j].Type {
			return sorted[i].Type < sorted[j].Type
		}
		return sorted[i].Port < sorted[j].Port
	})

	entries := make([]endpointSDEntry, 0, len(sorted))
	for _, ep := range sorted {
		entries = append(entries, ep.sdEntry())
	}

	out, err := yaml.Marshal(endpointSDConfig{Endpoints: entries})
	if err != nil {
		panic(err)
	}
	return string(out)
}

func newEndpointSDConfigMap(opts Options, objectMetaLabels map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        opts.GetGeneratedResourceName(),
			Namespace:   opts.Namespace,
			Labels:      objectMetaLabels,
			Annotations: opts.Annotations,
		},
		Data: map[string]string{
			endpointSDConfigKey: renderEndpointSDConfig(opts.Endpoints),
		},
	}
}
