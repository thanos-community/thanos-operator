package coretls

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/golang/snappy"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/prometheus/util/fmtutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/receive"
	"github.com/thanos-community/thanos-operator/test/utils"
)

// Port-forwarding changes the address, but TLS still verifies the Service identity.
func request(namespace, service string, remotePort int32, path string, body []byte, serverName string) ([]byte, error) {
	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: service, Namespace: namespace}, svc); err != nil {
		return nil, err
	}
	pods := &corev1.PodList{}
	if err := c.List(ctx, pods, client.InNamespace(namespace), client.MatchingLabels(svc.Spec.Selector)); err != nil {
		return nil, err
	}
	podName := ""
	for _, pod := range pods.Items {
		if pod.DeletionTimestamp != nil {
			continue
		}
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
				podName = pod.Name
				break
			}
		}
		if podName != "" {
			break
		}
	}
	if podName == "" {
		return nil, fmt.Errorf("no ready pod for %s", service)
	}
	port, cancel, err := utils.StartPortForward(ctx, intstr.FromInt32(remotePort), "https", podName, namespace)
	if err != nil {
		return nil, err
	}
	defer cancel()
	ca := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Name: featuregate.TLSCAName, Namespace: namespace}, ca); err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(ca.Data[featuregate.TLSCAKey])) {
		return nil, fmt.Errorf("invalid CA")
	}
	if serverName == "" {
		serverName = manifests.ServiceDNSName(service, namespace)
	}
	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, ServerName: serverName, MinVersion: tls.VersionTLS12}}
	defer transport.CloseIdleConnections()
	method := http.MethodGet
	if body != nil {
		method = http.MethodPost
	}
	scheme := "https"
	if serverName == "plaintext" {
		scheme = "http"
	}
	req, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("%s://127.0.0.1:%d%s", scheme, port, path), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/x-protobuf")
		req.Header.Set("Content-Encoding", "snappy")
		req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
	}
	response, err := (&http.Client{Transport: transport, Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	data, err := io.ReadAll(response.Body)
	if response.StatusCode/100 != 2 {
		return nil, fmt.Errorf("%s: %s", response.Status, string(data))
	}
	return data, err
}

func remoteWrite(namespace, service string, write utils.RemoteWriteRequest) error {
	data, err := fmtutil.MetricTextToWriteRequest(bytes.NewReader(write.Data), write.Labels)
	if err != nil {
		return err
	}
	raw, err := data.Marshal()
	if err != nil {
		return err
	}
	_, err = request(namespace, service, receive.RemoteWritePort, "/api/v1/receive", snappy.Encode(nil, raw), "")
	return err
}

func queryMetric(namespace, service, metric string, want int) error {
	data, err := request(namespace, service, 9090, "/api/v1/query?query="+url.QueryEscape(metric), nil, "")
	if err != nil {
		return err
	}
	var result struct {
		Data struct{ Result []struct{ Value []any } }
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return err
	}
	if len(result.Data.Result) == 0 {
		return fmt.Errorf("no results for %s", metric)
	}
	for _, series := range result.Data.Result {
		if len(series.Value) == 2 && fmt.Sprint(series.Value[1]) == fmt.Sprint(want) {
			return nil
		}
	}
	return fmt.Errorf("unexpected result for %s: %s", metric, string(data))
}

//nolint:tagliatelle // Thanos configuration uses snake_case.
func expectQueryDiscovery(namespace, query string, services ...string) {
	Eventually(func(g Gomega) {
		deployment := &appsv1.Deployment{}
		g.Expect(c.Get(ctx, client.ObjectKey{Name: query, Namespace: namespace}, deployment)).To(Succeed())
		g.Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(ContainElement("--endpoint.sd-config-file=" + manifests.TLSMountPath + "/endpoints/endpoints.yaml"))
		configMap := &corev1.ConfigMap{}
		g.Expect(c.Get(ctx, client.ObjectKey{Name: query, Namespace: namespace}, configMap)).To(Succeed())
		var config struct {
			DefaultClientConfig struct {
				TLSConfig struct {
					Enabled bool   `json:"enabled"`
					CAFile  string `json:"ca_file"`
				} `json:"tls_config"`
			} `json:"default_client_config"`
			Endpoints []struct {
				Address      string
				ClientConfig struct {
					ServerName string `json:"server_name"`
				} `json:"client_config"`
			}
		}
		g.Expect(json.Unmarshal([]byte(configMap.Data["endpoints.yaml"]), &config)).To(Succeed())
		g.Expect(config.DefaultClientConfig.TLSConfig.Enabled).To(BeTrue())
		g.Expect(config.DefaultClientConfig.TLSConfig.CAFile).To(Equal(manifests.TLSCAFile))
		g.Expect(config.Endpoints).To(HaveLen(len(services)))
		addresses := make(map[string]string, len(config.Endpoints))
		for _, endpoint := range config.Endpoints {
			addresses[endpoint.ClientConfig.ServerName] = endpoint.Address
		}
		for _, service := range services {
			pod := &corev1.Pod{}
			g.Expect(c.Get(ctx, client.ObjectKey{Name: service + "-0", Namespace: namespace}, pod)).To(Succeed())
			g.Expect(pod.Status.PodIP).NotTo(BeEmpty())
			g.Expect(addresses).To(HaveKeyWithValue(manifests.ServiceDNSName(service, namespace), net.JoinHostPort(pod.Status.PodIP, "10901")))
		}
	}, 3*time.Minute, time.Second).Should(Succeed())
}

func expectRulerDiscovery(namespace, ruler, query string) {
	serverName := manifests.ServiceDNSName(query, namespace)
	expected := fmt.Sprintf(`[{
		"scheme": "https",
		"static_configs": ["dnssrv+_http._tcp.%s"],
		"http_config": {
			"tls_config": {
				"ca_file": "%s",
				"server_name": "%s"
			}
		}
	}]`, serverName, manifests.TLSCAFile, serverName)
	Eventually(func(g Gomega) {
		workload := &appsv1.StatefulSet{}
		g.Expect(c.Get(ctx, client.ObjectKey{Name: ruler, Namespace: namespace}, workload)).To(Succeed())
		config := ""
		for _, arg := range workload.Spec.Template.Spec.Containers[0].Args {
			if strings.HasPrefix(arg, "--query.config=") {
				config = strings.TrimPrefix(arg, "--query.config=")
			}
		}
		g.Expect(config).NotTo(BeEmpty())
		g.Expect(config).To(MatchJSON(expected))
	}, 3*time.Minute, time.Second).Should(Succeed())
}

func expectTLSWorkload(namespace, name string, port int32) {
	Eventually(func(g Gomega) {
		cert := &cmv1.Certificate{}
		g.Expect(c.Get(ctx, client.ObjectKey{Name: manifests.TLSResourceName(name), Namespace: namespace}, cert)).To(Succeed())
		ready := false
		for _, condition := range cert.Status.Conditions {
			if condition.Type == cmv1.CertificateConditionReady && condition.Status == cmmeta.ConditionTrue {
				ready = true
			}
		}
		g.Expect(ready).To(BeTrue(), name)
		g.Expect(cert.Spec.DNSNames).To(ContainElement(manifests.ServiceDNSName(name, namespace)))
		g.Expect(cert.Spec.IssuerRef.Name).To(Equal(featuregate.TLSCAName))
		monitor := &monitoringv1.ServiceMonitor{}
		g.Expect(c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, monitor)).To(Succeed())
		g.Expect(monitor.Spec.Endpoints).To(HaveLen(1))
		endpoint := monitor.Spec.Endpoints[0]
		g.Expect(endpoint.Scheme).To(Equal(new(monitoringv1.SchemeHTTPS)))
		g.Expect(endpoint.TLSConfig).NotTo(BeNil())
		g.Expect(endpoint.TLSConfig.CA.ConfigMap).NotTo(BeNil())
		g.Expect(endpoint.TLSConfig.CA.ConfigMap.Name).To(Equal(featuregate.TLSCAName))
		g.Expect(endpoint.TLSConfig.CA.ConfigMap.Key).To(Equal(featuregate.TLSCAKey))
		g.Expect(endpoint.TLSConfig.ServerName).To(Equal(new(manifests.ServiceDNSName(name, namespace))))
		_, err := request(namespace, name, port, "/-/healthy", nil, "")
		g.Expect(err).NotTo(HaveOccurred())
	}, 3*time.Minute, time.Second).Should(Succeed())
}

func expectPrometheusTargets(namespace string, want int) {
	pods := &corev1.PodList{}
	Expect(c.List(ctx, pods, client.InNamespace("default"), client.MatchingLabels{"prometheus": "test-prometheus"})).To(Succeed())
	Expect(pods.Items).NotTo(BeEmpty())
	port, cancel, err := utils.StartPortForward(ctx, intstr.FromInt32(9090), "https", pods.Items[0].Name, "default")
	Expect(err).NotTo(HaveOccurred())
	defer cancel()
	Eventually(func() error {
		response, err := utils.QueryPrometheus(`up{namespace="`+namespace+`"}`, port)
		if err != nil {
			return err
		}
		if len(response.Data.Result) != want {
			return fmt.Errorf("expected %d scrape targets, got %d", want, len(response.Data.Result))
		}
		for _, result := range response.Data.Result {
			if fmt.Sprint(result.Value[1]) != "1" {
				return fmt.Errorf("scrape target is down: %v", result)
			}
		}
		return nil
	}, 3*time.Minute, 2*time.Second).Should(Succeed())
}
