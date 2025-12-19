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

package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2" //nolint:golint,revive

	"github.com/golang/snappy"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	pconf "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/storage/remote"
	"github.com/prometheus/prometheus/util/fmtutil"
	"gotest.tools/v3/golden"

	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/yaml"
)

const (
	prometheusOperatorVersion = "v0.87.0"
	prometheusOperatorURL     = "https://github.com/prometheus-operator/prometheus-operator/" +
		"releases/download/%s/bundle.yaml"

	certmanagerVersion = "v1.5.3"
	certmanagerURLTmpl = "https://github.com/jetstack/cert-manager/releases/download/%s/cert-manager.yaml"
)

type PrometheusResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []any             `json:"value"` // Use interface{} because value can be mixed types
		} `json:"result"`
	} `json:"data"`
}

func warnError(err error) {
	_, _ = fmt.Fprintf(GinkgoWriter, "warning: %v\n", err)
}

// InstallPrometheusOperator installs the prometheus Operator to be used to export the enabled metrics.
func InstallPrometheusOperator() error {
	url := fmt.Sprintf(prometheusOperatorURL, prometheusOperatorVersion)
	cmd := exec.Command("kubectl", "apply", "--server-side", "-f", url)
	_, err := Run(cmd)
	return err
}

// Run executes the provided command within this context
func Run(cmd *exec.Cmd) ([]byte, error) {
	dir, _ := GetProjectDir()
	cmd.Dir = dir

	if err := os.Chdir(cmd.Dir); err != nil {
		_, _ = fmt.Fprintf(GinkgoWriter, "chdir dir: %s\n", err)
	}

	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	command := strings.Join(cmd.Args, " ")
	_, _ = fmt.Fprintf(GinkgoWriter, "running: %s\n", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("%s failed with error: (%v) %s", command, err, string(output))
	}

	return output, nil
}

// UninstallPrometheusOperator uninstalls the prometheus
func UninstallPrometheusOperator() {
	url := fmt.Sprintf(prometheusOperatorURL, prometheusOperatorVersion)
	cmd := exec.Command("kubectl", "delete", "-f", url)
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}
}

// UninstallCertManager uninstalls the cert manager
func UninstallCertManager() {
	url := fmt.Sprintf(certmanagerURLTmpl, certmanagerVersion)
	cmd := exec.Command("kubectl", "delete", "-f", url)
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}
}

// InstallCertManager installs the cert manager bundle.
func InstallCertManager() error {
	url := fmt.Sprintf(certmanagerURLTmpl, certmanagerVersion)
	cmd := exec.Command("kubectl", "apply", "-f", url)
	if _, err := Run(cmd); err != nil {
		return err
	}
	// Wait for cert-manager-webhook to be ready, which can take time if cert-manager
	// was re-installed after uninstalling on a cluster.
	cmd = exec.Command("kubectl", "wait", "deployment.apps/cert-manager-webhook",
		"--for", "condition=Available",
		"--namespace", "cert-manager",
		"--timeout", "5m",
	)

	_, err := Run(cmd)
	return err
}

// LoadImageToKindClusterWithName  loads a local docker image to the kind cluster
func LoadImageToKindClusterWithName(name string) error {
	cluster := "kind"
	if v, ok := os.LookupEnv("KIND_CLUSTER"); ok {
		cluster = v
	}
	kindOptions := []string{"load", "docker-image", name, "--name", cluster}
	cmd := exec.Command("kind", kindOptions...)
	_, err := Run(cmd)
	return err
}

// GetNonEmptyLines converts given command output string into individual objects
// according to line breakers, and ignores the empty elements in it.
func GetNonEmptyLines(output string) []string {
	var res []string
	elements := strings.SplitSeq(output, "\n")
	for element := range elements {
		if element != "" {
			res = append(res, element)
		}
	}

	return res
}

// GetProjectDir will return the directory where the project is
func GetProjectDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return wd, err
	}
	wd = strings.Replace(wd, "/test/e2e", "", -1)
	return wd, nil
}

// InstallMinIO installs the object store
func InstallMinIO() error {
	cmd := exec.Command("kubectl", "apply", "-f", minioTestData())
	_, err := Run(cmd)
	return err
}

// UninstallMinIO uninstalls the object store
func UninstallMinIO() {
	cmd := exec.Command("kubectl", "delete", "-f", minioTestData())
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}
}

func CreateMinioObjectStorageSecret() error {
	wd, _ := os.Getwd()
	path := wd + "/test/utils/testdata/minio-secret.yaml"
	cmd := exec.Command("kubectl", "apply", "-f", path)
	_, err := Run(cmd)
	return err
}

type Verifier struct {
	cm, secret, svc, sa, sts, dep, svcMon bool
}

func (v Verifier) WithConfigMap() Verifier {
	v.cm = true
	return v
}

func (v Verifier) WithSecret() Verifier {
	v.secret = true
	return v
}

func (v Verifier) WithService() Verifier {
	v.svc = true
	return v
}

func (v Verifier) WithStatefulSet() Verifier {
	v.sts = true
	return v
}

func (v Verifier) WithDeployment() Verifier {
	v.dep = true
	return v
}

func (v Verifier) WithServiceMonitor() Verifier {
	v.svcMon = true
	return v
}

func (v Verifier) WithServiceAccount() Verifier {
	v.sa = true
	return v
}

func (v Verifier) Verify(c client.Client, name, ns string) bool {
	if v.cm && !VerifyConfigMapExists(c, name, ns) {
		return false
	}
	if v.secret && !VerifySecretExists(c, name, ns) {
		return false
	}
	if v.svc && !VerifyServiceExists(c, name, ns) {
		return false
	}
	if v.sts && !VerifyStatefulSetExists(c, name, ns) {
		return false
	}
	if v.dep && !VerifyDeploymentExists(c, name, ns) {
		return false
	}
	if v.svcMon && !VerifyServiceMonitorExists(c, name, ns) {
		return false
	}
	return true
}

func VerifyConfigMapExists(c client.Client, name, namespace string) bool {
	cm := &corev1.ConfigMap{}
	err := c.Get(context.Background(), client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, cm)
	return err == nil
}

func VerifyServiceExists(c client.Client, name string, namespace string) bool {
	svc := &corev1.Service{}
	err := c.Get(context.Background(), client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, svc)
	return err == nil
}

func VerifyServiceAccountExists(c client.Client, name string, namespace string) bool {
	sa := &corev1.ServiceAccount{}
	err := c.Get(context.Background(), client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, sa)
	return err == nil
}

func VerifyDeploymentExists(c client.Client, name string, namespace string) bool {
	deployment := &appsv1.Deployment{}
	err := c.Get(context.Background(), client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, deployment)
	return err == nil
}

func VerifySecretExists(c client.Client, name string, namespace string) bool {
	secret := &corev1.Secret{}
	err := c.Get(context.Background(), client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, secret)
	return err == nil
}

func VerifyServiceMonitorExists(c client.Client, name, ns string) bool {
	sm := &monitoringv1.ServiceMonitor{}
	err := c.Get(context.Background(), client.ObjectKey{
		Name:      name,
		Namespace: ns,
	}, sm)
	return err == nil
}

func VerifyStatefulSetReplicasRunning(c client.Client, expect int, name string, namespace string) bool {
	sts := &appsv1.StatefulSet{}
	err := c.Get(context.Background(), client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, sts)
	if err != nil {
		return false
	}
	if *sts.Spec.Replicas != int32(expect) {
		return false
	}
	if sts.Status.ReadyReplicas != int32(expect) {
		return false
	}
	return true
}

func VerifyStatefulSetReplicas(c client.Client, expect int, name string, namespace string) bool {
	sts := &appsv1.StatefulSet{}
	err := c.Get(context.Background(), client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, sts)
	if err != nil {
		return false
	}
	if *sts.Spec.Replicas != int32(expect) {
		return false
	}
	return true
}

func VerifyDeploymentReplicasRunning(c client.Client, expect int, name string, namespace string) bool {
	deployment := &appsv1.Deployment{}
	err := c.Get(context.Background(), client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, deployment)
	if err != nil {
		return false
	}
	if *deployment.Spec.Replicas != int32(expect) {
		return false
	}
	if deployment.Status.ReadyReplicas != int32(expect) {
		return false
	}
	return true
}

func VerifyDeploymentArgs(c client.Client, name string, namespace string, containerIndex int, containsArg string) bool {
	deployment := &appsv1.Deployment{}
	err := c.Get(context.Background(), client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, deployment)
	if err != nil {
		return false
	}

	return slices.Contains(deployment.Spec.Template.Spec.Containers[containerIndex].Args, containsArg)
}

func VerifyStatefulSetArgs(
	c client.Client,
	name string,
	namespace string,
	containerIndex int,
	containsArg string,
) bool {
	statefulset := &appsv1.StatefulSet{}
	err := c.Get(context.Background(), client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, statefulset)
	if err != nil {
		return false
	}

	return slices.Contains(statefulset.Spec.Template.Spec.Containers[containerIndex].Args, containsArg)
}

func VerifyStatefulSetExists(c client.Client, name string, namespace string) bool {
	sts := &appsv1.StatefulSet{}
	err := c.Get(context.Background(), client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, sts)
	return err == nil
}

func VerifyPodDisruptionBudgetExists(c client.Client, name string, namespace string) bool {
	pdb := &policyv1.PodDisruptionBudget{}
	err := c.Get(context.Background(), client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, pdb)
	return err == nil
}

func VerifyConfigMapContents(c client.Client, name, namespace, key, expect string) bool {
	cm := &corev1.ConfigMap{}
	if err := c.Get(context.Background(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, cm); err != nil {
		return false
	}

	data := cm.Data[key]

	if json.Valid([]byte(data)) && json.Valid([]byte(expect)) {
		var jData any
		if err := json.Unmarshal([]byte(data), &jData); err != nil {
			return false
		}
		var jExpect any
		if err := json.Unmarshal([]byte(expect), &jExpect); err != nil {
			return false
		}
		return equality.Semantic.DeepEqual(jData, jExpect)

	}
	return data == expect
}

// StartPortForward initiates a port forwarding connection to a pod on the
// localhost interface. It returns a closer function that should be invoked to
// stop the proxy server.
// The function blocks until the port forwarding proxy server is ready to
// receive connections or the context is canceled.
func StartPortForward(ctx context.Context, port intstr.IntOrString, scheme, name, ns string) (func(), error) {
	conf, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	roundTripper, upgrader, err := spdy.RoundTripperFor(conf)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", ns, name)
	hostIP := strings.TrimLeft(conf.Host, "htps:/")
	serverURL := url.URL{Scheme: scheme, Path: path, Host: hostIP}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, http.MethodPost, &serverURL)

	stopChan, readyChan := make(chan struct{}, 1), make(chan struct{}, 1)
	out, errOut := new(bytes.Buffer), new(bytes.Buffer)
	forwarder, err := portforward.New(dialer, []string{port.String()}, stopChan, readyChan, out, errOut)
	if err != nil {
		return nil, err
	}

	forwardErr := make(chan error, 1)
	go func() {
		if err := forwarder.ForwardPorts(); err != nil {
			forwardErr <- err
		}
		close(forwardErr)
	}()

	select {
	case <-readyChan:
		return func() { close(stopChan) }, nil
	case <-ctx.Done():
		var err error
		select {
		case err = <-forwardErr:
		default:
		}
		return nil, fmt.Errorf("%v: %v", ctx.Err(), err)
	}
}

func minioTestData() string {
	wd, _ := os.Getwd()
	return wd + "/test/utils/testdata/minio.yaml"
}

// RemoteWrite sends a remote write request to the remote write endpoint which is running on localhost.
func RemoteWrite(req RemoteWriteRequest, roundTripper http.RoundTripper, headers map[string]string) error {
	url, err := url.Parse("http://localhost:19291/api/v1/receive")
	if err != nil {
		return err
	}

	rwClient, err := remote.NewWriteClient("test-client", &remote.ClientConfig{
		Timeout: model.Duration(time.Second * 5),
		URL:     &pconf.URL{URL: url},
		HTTPClientConfig: pconf.HTTPClientConfig{
			TLSConfig: pconf.TLSConfig{
				InsecureSkipVerify: true,
			},
		},
	})
	if err != nil {
		return err
	}

	writeClient, ok := rwClient.(*remote.Client)
	if !ok {
		return fmt.Errorf("failed to cast remote write client")
	}

	if roundTripper == nil {
		roundTripper = http.DefaultTransport
	}

	writeClient.Client.Transport = &setHeadersTransport{
		RoundTripper: roundTripper,
		headers:      headers,
	}

	data, err := fmtutil.MetricTextToWriteRequest(bytes.NewReader(req.Data), req.Labels)
	if err != nil {
		return err
	}

	raw, err := data.Marshal()
	if err != nil {
		return err
	}

	compressed := snappy.Encode(nil, raw)
	_, err = writeClient.Store(context.Background(), compressed, 0)
	if err != nil {
		return err
	}
	return nil
}

// RemoteWriteRequest is a struct that represents the request to be sent to the remote write endpoint.
type RemoteWriteRequest struct {
	Data   []byte
	Labels map[string]string
}

// DefaultRemoteWriteRequest returns a default RemoteWriteRequest.
func DefaultRemoteWriteRequest() RemoteWriteRequest {
	return RemoteWriteRequest{
		Data: []byte(`
	# HELP test_metric This is a test metric.
	# TYPE test_metric gauge
	test_metric{foo="bar"} 1 1
	`),
		Labels: map[string]string{
			"job": "e2e-test",
		},
	}
}

type setHeadersTransport struct {
	http.RoundTripper
	headers map[string]string
}

func (s *setHeadersTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for key, value := range s.headers {
		req.Header.Set(key, value)
	}
	return s.RoundTripper.RoundTrip(req)
}

func VerifyCfgMapOrSecretEnvVarExists(c client.Client, obj client.Object, name, ns string, containerIndex int, envVarName string, key string, cfgOrSecret string) bool {
	switch obj.(type) {
	case *appsv1.Deployment:
		deployment := &appsv1.Deployment{}
		err := c.Get(context.Background(), client.ObjectKey{
			Name:      name,
			Namespace: ns,
		}, deployment)
		if err != nil {
			return false
		}

		for _, e := range deployment.Spec.Template.Spec.Containers[containerIndex].Env {
			if e.Name == envVarName {
				if e.ValueFrom.SecretKeyRef != nil {
					if e.ValueFrom.SecretKeyRef.Key == key && e.ValueFrom.SecretKeyRef.Name == cfgOrSecret {
						return true
					}
				} else if e.ValueFrom.ConfigMapKeyRef != nil {
					if e.ValueFrom.ConfigMapKeyRef.Key == key && e.ValueFrom.ConfigMapKeyRef.Name == cfgOrSecret {
						return true
					}
				}
				return false
			}
		}
		return false
	case *appsv1.StatefulSet:
		statefulSet := &appsv1.StatefulSet{}
		err := c.Get(context.Background(), client.ObjectKey{
			Name:      name,
			Namespace: ns,
		}, statefulSet)
		if err != nil {
			return false
		}

		for _, e := range statefulSet.Spec.Template.Spec.Containers[containerIndex].Env {
			if e.Name == envVarName {
				if e.ValueFrom.SecretKeyRef != nil {
					if e.ValueFrom.SecretKeyRef.Key == key && e.ValueFrom.SecretKeyRef.Name == cfgOrSecret {
						return true
					}
				} else if e.ValueFrom.ConfigMapKeyRef != nil {
					if e.ValueFrom.ConfigMapKeyRef.Key == key && e.ValueFrom.ConfigMapKeyRef.Name == cfgOrSecret {
						return true
					}
				}
				return false
			}
		}
		return false
	default:
		return false
	}
}

func ValidateObjectsEqual(t *testing.T, got client.Object, want client.Object) {
	t.Helper()
	if got.GetObjectKind().GroupVersionKind() != want.GetObjectKind().GroupVersionKind() {
		t.Errorf("got GVK %s, want %s", got.GetObjectKind().GroupVersionKind(), want.GetObjectKind().GroupVersionKind())
	}

	if !equality.Semantic.DeepEqual(got, want) {
		t.Errorf("got %v, want %v for object %s %s", got, want, got.GetObjectKind().GroupVersionKind().Kind, got.GetName())
	}
}

func ValidateObjectLabelsEqual(t *testing.T, wantLabels map[string]string, onObjs ...client.Object) {
	t.Helper()
	for _, obj := range onObjs {
		if !equality.Semantic.DeepEqual(obj.GetLabels(), wantLabels) {
			t.Errorf("expected object %s of kind %s to have labels %v, got %v",
				obj.GetName(), obj.GetObjectKind().GroupVersionKind().Kind, wantLabels, obj.GetLabels())
		}
	}
}

func ValidateHasLabels(t *testing.T, obj client.Object, labels map[string]string) {
	t.Helper()
	for k, v := range labels {
		if obj.GetLabels()[k] != v {
			t.Errorf("expected object %s to have label %s with value %s, got %s",
				obj.GetName(), k, v, obj.GetLabels()[k])
		}
	}
}

func ValidateLabelsMatch(t *testing.T, objWithSelector client.Object, matches client.Object) {
	t.Helper()
	var ls *metav1.LabelSelector

	switch objWithSelector := objWithSelector.(type) {
	case *corev1.Service:
		s := objWithSelector.Spec.Selector
		ls = &metav1.LabelSelector{MatchLabels: s}
	case *policyv1.PodDisruptionBudget:
		ls = objWithSelector.Spec.Selector
	case *monitoringv1.ServiceMonitor:
		s := objWithSelector.Spec.Selector
		ls = &s
	default:
		return
	}

	selector, err := metav1.LabelSelectorAsSelector(ls)
	if err != nil {
		t.Errorf("failed to convert label selector to selector: %v", err)
	}
	if !selector.Matches(labels.Set(matches.GetLabels())) {
		t.Errorf("expected object %s to match labels %v, got %v",
			objWithSelector.GetName(), ls, matches.GetLabels())
	}
}

func ValidateNameAndNamespace(t *testing.T, obj client.Object, name, namespace string) {
	t.Helper()
	if obj.GetName() != name {
		t.Errorf("expected object to have name %s, got %s", name, obj.GetName())
	}
	if obj.GetNamespace() != namespace {
		t.Errorf("expected object to have namespace %s, got %s", namespace, obj.GetNamespace())
	}
}

func ValidateNameNamespaceAndLabels(t *testing.T, obj client.Object, name, namespace string, labels map[string]string) {
	t.Helper()
	ValidateNameAndNamespace(t, obj, name, namespace)
	ValidateObjectLabelsEqual(t, labels, obj)
}

func SetUpPrometheus(c client.Client) error {
	if err := CreateServiceAccount(c); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	if err := CreateClusterRole(c); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	if err := CreateClusterRoleBinding(c); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	if err := CreatePrometheus(c); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func CreatePrometheus(c client.Client) error {
	promRes := &monitoringv1.Prometheus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-prometheus",
			Namespace: "default",
		},
		Spec: monitoringv1.PrometheusSpec{
			CommonPrometheusFields: monitoringv1.CommonPrometheusFields{
				ServiceAccountName:              "prometheus",
				Replicas:                        ptr.To(int32(1)),
				ServiceMonitorNamespaceSelector: &metav1.LabelSelector{},
				ServiceMonitorSelector:          &metav1.LabelSelector{},
				Version:                         "v2.54.0",
			},
			Retention: "10d",
		},
	}
	if err := c.Create(context.Background(), promRes); err != nil {
		return err
	}
	return nil
}

// create service account for prometheus
func CreateServiceAccount(c client.Client) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prometheus",
			Namespace: "default",
		},
	}
	if err := c.Create(context.Background(), sa); err != nil {
		return err
	}
	return nil
}

func CreateClusterRole(c client.Client) error {
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "prometheus",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"nodes", "nodes/proxy", "services", "endpoints", "pods"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"get"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get"},
			},
			{
				NonResourceURLs: []string{"/metrics"},
				Verbs:           []string{"get"},
			},
		},
	}
	if err := c.Create(context.Background(), cr); err != nil {
		return err
	}
	return nil
}

func CreateClusterRoleBinding(c client.Client) error {
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "prometheus",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "prometheus",
				Namespace: "default",
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind: "ClusterRole",
			Name: "prometheus",
		},
	}
	if err := c.Create(context.Background(), crb); err != nil {
		return err
	}
	return nil
}

func QueryPrometheus(query string) (*PrometheusResponse, error) {
	url := fmt.Sprintf("http://localhost:9090/api/v1/query?query=%s", query)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	var promResp PrometheusResponse
	if err := json.NewDecoder(resp.Body).Decode(&promResp); err != nil {
		return nil, err
	}
	return &promResp, nil
}

func ValidateIsNamedServiceAccount(t *testing.T, obj client.Object, b manifests.Buildable, namespace string) {
	t.Helper()
	if obj.GetObjectKind().GroupVersionKind().Kind != "ServiceAccount" {
		t.Errorf("expected object to be a service account, got %v", obj.GetObjectKind().GroupVersionKind().Kind)
	}

	ValidateNameNamespaceAndLabels(t, obj, b.GetGeneratedResourceName(), namespace, b.GetSelectorLabels())
}

func ValidateIsNamedPodDisruptionBudget(t *testing.T, obj client.Object, b manifests.Buildable, namespace string, matching client.Object) {
	t.Helper()
	if obj.GetObjectKind().GroupVersionKind().Kind != "PodDisruptionBudget" {
		t.Errorf("expected object to be a PodDisruptionBudget, got %v", obj.GetObjectKind().GroupVersionKind().Kind)
	}
	ValidateNameAndNamespace(t, obj, b.GetGeneratedResourceName(), namespace)
	ValidateLabelsMatch(t, obj, matching)
}

// ValidateObjectAgainstGoldenFile compares a Kubernetes object against a golden YAML file
// using gotest.tools/v3/golden for consistent behavior with prometheus-operator.
func ValidateObjectAgainstGoldenFile(t *testing.T, obj client.Object, goldenFileName string) {
	t.Helper()

	yamlBytes, err := yaml.Marshal(obj)
	if err != nil {
		t.Fatalf("failed to marshal object to YAML: %v", err)
	}

	golden.Assert(t, string(yamlBytes), goldenFileName)
}

// ValidateObjectsAgainstGoldenFile validates multiple objects against a single golden file
// containing multiple YAML documents separated by "---"
func ValidateObjectsAgainstGoldenFile(t *testing.T, objs []client.Object, goldenFileName string) {
	t.Helper()

	var yamlDocs []string
	for _, obj := range objs {
		yamlBytes, err := yaml.Marshal(obj)
		if err != nil {
			t.Fatalf("failed to marshal object to YAML: %v", err)
		}
		yamlDocs = append(yamlDocs, string(yamlBytes))
	}
	
	combinedYAML := strings.Join(yamlDocs, "---\n")
	golden.Assert(t, combinedYAML, goldenFileName)
}
