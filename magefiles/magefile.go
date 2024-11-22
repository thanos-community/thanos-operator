//go:build mage
// +build mage

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/magefile/mage/sh"
	"github.com/thanos-community/thanos-operator/test/utils"
)

const (
	kindClusterName   = "thanos-operator-cluster"
	operatorNamespace = "thanos-operator-system"
	projectImage      = "quay.io/thanos/thanos-operator"
)

// InteractiveDemo runs an interactive demo.
func InteractiveDemo() error {
	if err := KindCluster(); err != nil {
		return err
	}

	if err := os.Setenv("KIND_CLUSTER", kindClusterName); err != nil {
		return err
	}

	if err := PrometheusOperator(); err != nil {
		return err
	}

	if err := configurePrometheusInteractiveDemo(); err != nil {
		return err
	}

	if err := createOperatorNamespace(); err != nil {
		return err
	}

	if err := Minio(); err != nil {
		return err
	}

	if err := BuildAndLoadLocalImage(); err != nil {
		return err
	}

	if err := InstallCRDS(); err != nil {
		return err
	}

	if err := InstallSamples(); err != nil {
		return err
	}
	return nil
}

// Creates a new KinD cluster named 'thanos-operator-cluster'
func KindCluster() error {
	o, err := sh.Output("kind", "get", "clusters")
	if err != nil {
		return fmt.Errorf("failed to get KinD clusters: %w", err)
	}

	if strings.Contains(o, kindClusterName) {
		log.Println("KinD cluster already exists, skipping creation...")
		return nil
	}

	return sh.Run("kind", "create", "cluster", "--name", kindClusterName)
}

// Installs the Prometheus Operator in the Kubernetes cluster
func PrometheusOperator() error {
	return utils.InstallPrometheusOperator()
}

// Installs MinIO in the Kubernetes cluster
func Minio() error {
	err := utils.InstallMinIO()
	if err != nil {
		return err
	}
	return utils.CreateMinioObjectStorageSecret()
}

// Builds the container image and loads it into the KinD cluster
func BuildAndLoadLocalImage() error {
	err := BuildLocalImage()
	if err != nil {
		return err
	}
	return utils.LoadImageToKindClusterWithName(getImageName())
}

// Builds the container image
func BuildLocalImage() error {
	return sh.Run("docker", "build", ".", "-t", getImageName())
}

// Applies the Thanos Operators CRDs into the Kubernetes cluster
func InstallCRDS() error {
	return sh.Run("kubectl", "apply", "--server-side", "-f", "config/crd/bases/")
}

// Installs the Thanos Operator sample resources into the Kubernetes cluster
func InstallSamples() error {
	if err := sh.Run("make", "install-sample"); err != nil {
		return err
	}
	return nil
}

func createOperatorNamespace() error {
	content := fmt.Sprintf(`
kind: Namespace
apiVersion: v1
metadata:
  name: %s
`, operatorNamespace)

	_, err := applyKubeResources(content)
	return err
}

func getImageName() string {
	x, err := sh.Output("git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		panic(err)
	}
	y, err := sh.Output("date", "+%Y-%m-%d")
	if err != nil {
		panic(err)
	}
	z, err := sh.Output("git", "rev-parse", "--short", "HEAD")
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s:%s-%s-%s", projectImage, x, y, z)
}

func applyKubeResources(resources string, withArgs ...string) (string, error) {
	in := []byte(resources)
	dir, err := os.MkdirTemp("", "resources")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	file := filepath.Join(dir, "tmpfile")
	if err := os.WriteFile(file, in, 0666); err != nil {
		return "", err
	}
	args := append([]string{"apply", "-f", file}, withArgs...)
	return sh.Output("kubectl", args...)
}

func applyNamespacedKubeResources(resources, namespace string) (string, error) {
	in := []byte(resources)
	dir, err := os.MkdirTemp("", "resources")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	file := filepath.Join(dir, "tmpfile")
	if err := os.WriteFile(file, in, 0666); err != nil {
		return "", err
	}
	return sh.Output("kubectl", "-n", namespace, "apply", "-f", file)
}

func configurePrometheusInteractiveDemo() error {
	content := `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: prometheus
rules:
- apiGroups: [""]
  resources:
  - nodes
  - nodes/metrics
  - services
  - endpoints
  - pods
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources:
  - configmaps
  verbs: ["get"]
- apiGroups:
  - discovery.k8s.io
  resources:
  - endpointslices
  verbs: ["get", "list", "watch"]
- apiGroups:
  - networking.k8s.io
  resources:
  - ingresses
  verbs: ["get", "list", "watch"]
- nonResourceURLs: ["/metrics"]
  verbs: ["get"]
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: prometheus
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: prometheus
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: prometheus
subjects:
- kind: ServiceAccount
  name: prometheus
  namespace: default
---
apiVersion: monitoring.coreos.com/v1
kind: Prometheus
metadata:
  name: prometheus-dev
spec:
  serviceAccountName: prometheus
  serviceMonitorNamespaceSelector:
    matchLabels:
      kubernetes.io/metadata.name: thanos-operator-system
  serviceMonitorSelector:
    matchExpressions:
    - {key: 'app.kubernetes.io/part-of', operator: In, values: [thanos, thanos-operator]}
  remoteWrite:
    - url: http://thanos-receive-router-example-receive.thanos-operator-system.svc.cluster.local:19291/api/v1/receive
      name: thanos-receive-router-green
`
	_, err := applyNamespacedKubeResources(content, "default")
	return err
}
