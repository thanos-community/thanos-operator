package manifests_test

import (
	"strings"
	"testing"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/compact"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/query"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/queryfrontend"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/receive"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/ruler"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/store"
)

func TestTLSComponents(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		opts := manifests.Options{Owner: "test", Namespace: "test", Config: featuregate.Config{
			ServerTLS:      &featuregate.ServerTLSConfig{FeatureConfig: featuregate.FeatureConfig{Enabled: enabled}},
			ServiceMonitor: &featuregate.ServiceMonitorConfig{FeatureConfig: featuregate.FeatureConfig{Enabled: true}},
		}}
		for _, b := range []manifests.Buildable{
			query.Options{Options: opts}, queryfrontend.Options{Options: opts}, store.Options{Options: opts},
			receive.IngesterOptions{Options: opts}, receive.RouterOptions{Options: opts}, ruler.Options{Options: opts}, compact.Options{Options: opts},
		} {
			t.Run(b.GetGeneratedResourceName()+"/tls="+map[bool]string{true: "true", false: "false"}[enabled], func(t *testing.T) {
				for _, obj := range b.Build() {
					if sm, ok := obj.(*monitoringv1.ServiceMonitor); ok {
						ep := sm.Spec.Endpoints[0]
						if enabled {
							require.Equal(t, ptr.To(monitoringv1.SchemeHTTPS), ep.Scheme)
							require.Equal(t, manifests.ServiceDNSName(sm.Name, sm.Namespace), *ep.TLSConfig.ServerName)
							require.Equal(t, featuregate.TLSCAName, ep.TLSConfig.CA.ConfigMap.Name)
							require.Empty(t, ep.TLSConfig.Cert)
						} else {
							require.Nil(t, ep.TLSConfig)
						}
					}
					pod := manifests.PodTemplate(obj)
					if pod == nil {
						continue
					}
					c := pod.Spec.Containers[0]
					args := strings.Join(c.Args, " ")
					require.Equal(t, enabled, strings.Contains(args, "--http.config="))
					if !enabled {
						continue
					}
					require.NoError(t, manifests.ValidateTLSWorkload(pod))
					for _, probe := range []*corev1.Probe{c.StartupProbe, c.ReadinessProbe, c.LivenessProbe} {
						if probe != nil && probe.HTTPGet != nil {
							require.Equal(t, corev1.URISchemeHTTPS, probe.HTTPGet.Scheme)
						}
					}
					require.NotContains(t, args, "skip-verify")
					require.NotContains(t, args, "server-tls-client-ca")
					if c.Args[0] == "receive" {
						require.Contains(t, args, "--remote-write.client-tls-secure")
					}
				}
			})
		}
	}
}

func TestTLSDoesNotRestrictImages(t *testing.T) {
	flags := featuregate.Flag{featuregate.ServerTLS}
	for _, image := range []string{
		"quay.io/thanos/thanos:v0.41.0",
		"quay.io/thanos/thanos:v0.42.0",
		"quay.io/thanos/thanos:latest",
		"quay.io/thanos/thanos:v0.42.0-rc.0",
		"example.com/thanos:custom",
		"example.com/thanos@sha256:" + strings.Repeat("a", 64),
	} {
		t.Run(image, func(t *testing.T) {
			pod := manifests.PodTemplate(query.NewQueryDeployment(query.Options{Options: manifests.Options{Owner: "test", Image: &image, Config: flags.ToFeatureGate()}}))
			require.NoError(t, manifests.ValidateTLSWorkload(pod))
			require.Equal(t, image, pod.Spec.Containers[0].Image)
			require.Contains(t, pod.Spec.Containers[0].Args, "--http.config="+manifests.TLSWebConfigFile)
		})
	}
}

func TestTLSKeepsSidecarMetricsSeparate(t *testing.T) {
	flags := featuregate.Flag{featuregate.ServerTLS, featuregate.ServiceMonitor, featuregate.KubeResourceSync}
	opts := receive.RouterOptions{Options: manifests.Options{Owner: "test", Namespace: "metrics", Config: flags.ToFeatureGate()}}
	foundSidecar := false
	for _, obj := range opts.Build() {
		monitor, ok := obj.(*monitoringv1.ServiceMonitor)
		if !ok {
			continue
		}
		for _, ep := range monitor.Spec.Endpoints {
			if ep.Port == "kube-resource-sync" {
				foundSidecar = true
				require.Nil(t, ep.TLSConfig)
				require.Nil(t, ep.Scheme)
			} else {
				require.NotNil(t, ep.TLSConfig)
			}
		}
	}
	require.True(t, foundSidecar)
}
