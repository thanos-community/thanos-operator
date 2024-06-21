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
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2" //nolint:golint,revive

	"github.com/golang/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	prometheusOperatorVersion = "v0.68.0"
	prometheusOperatorURL     = "https://github.com/prometheus-operator/prometheus-operator/" +
		"releases/download/%s/bundle.yaml"

	certmanagerVersion = "v1.5.3"
	certmanagerURLTmpl = "https://github.com/jetstack/cert-manager/releases/download/%s/cert-manager.yaml"
)

func warnError(err error) {
	fmt.Fprintf(GinkgoWriter, "warning: %v\n", err)
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
		fmt.Fprintf(GinkgoWriter, "chdir dir: %s\n", err)
	}

	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	command := strings.Join(cmd.Args, " ")
	fmt.Fprintf(GinkgoWriter, "running: %s\n", command)
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
	elements := strings.Split(output, "\n")
	for _, element := range elements {
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

func VerifyStsReplicasRunning(c client.Client, expect int, name string, namespace string) bool {
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

func VerifyDeploymentExists(c client.Client, name string, namespace string) bool {
	deployment := &appsv1.Deployment{}
	err := c.Get(context.Background(), client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, deployment)

	return err == nil
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

func VerifyDeploymentArgs(c client.Client, name string, namespace string, containsArg string) bool {
	deployment := &appsv1.Deployment{}
	err := c.Get(context.Background(), client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, deployment)
	if err != nil {
		return false
	}

	return slices.Contains(deployment.Spec.Template.Spec.Containers[0].Args, containsArg)
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
	return data == expect
}

// StartPortForward initiates a port forwarding connection to a pod on the
// localhost interface. It returns a closer function that should be invoked to
// stop the proxy server.
// The function blocks until the port forwarding proxy server is ready to
// receive connections or the context is canceled.
func StartPortForward(ctx context.Context, scheme string, name string, ns string, port intstr.IntOrString) (func(), error) {
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

func RemoteWrite(tenant, url string) (status int) {
	req := &prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{
						Name:  "__name__",
						Value: "test_metric",
					},
					{
						Name:  "_id",
						Value: "true",
					},
				},
				Samples: []prompb.Sample{
					{
						Timestamp: time.Now().Unix(),
						Value:     1.0,
					},
				},
			},
		},
	}

	data, err := proto.Marshal(req)
	if err != nil {
		return status
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	body := bytes.NewReader(snappy.Encode(nil, data))
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return status
	}
	request.Header.Set("Content-Type", "application/x-protobuf")
	request.Header.Set("Content-Encoding", "snappy")
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return status
	}

	_ = resp.Body.Close()

	return resp.StatusCode
}

func minioTestData() string {
	wd, _ := os.Getwd()
	return wd + "/test/utils/testdata/minio.yaml"
}
