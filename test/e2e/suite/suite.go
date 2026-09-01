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

// Package suite provides the shared bootstrap for the e2e suites. The expensive
// one-time cluster setup (operator image, prometheus-operator, cert-manager,
// MinIO, test Prometheus) is done once by `make e2e-setup` before the suites run;
// each suite only calls Setup to get a client scoped to its own namespace against
// that shared cluster. Splitting per area lets the suites run as parallel `go test`
// binaries while ordering within a suite is preserved.
package suite

import (
	"context"
	"os"
	"time"

	"github.com/onsi/gomega"

	"k8s.io/utils/ptr"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	"github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/controller"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/test/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	resourceapi "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	// ObjStoreSecret is the name of the object-storage secret each suite creates in
	// its namespace, pointing at the shared MinIO.
	ObjStoreSecret = "thanos-object-storage"
	// ObjStoreSecretKey is the key holding the Thanos object-storage config.
	ObjStoreSecretKey = "thanos.yaml"
)

// objStoreConfig is the Thanos object-storage config pointing at the shared MinIO
// deployed by e2e-setup in thanos-operator-system. The endpoint is a fully
// qualified service DNS name so it is reachable from any namespace.
const objStoreConfig = `type: S3
config:
  bucket: "thanos"
  access_key: "thanos"
  secret_key: "thanos-secret"
  endpoint: "minio.thanos-operator-system.svc:9000"
  insecure: true
  trace:
    enable: false
`

// Setup builds a controller-runtime client, ensures the given namespace exists,
// and creates the object-storage secret in it. It returns the client, the
// namespace name, and a teardown func (deletes the namespace) intended to be
// called from AfterSuite. The one-time cluster bootstrap is assumed to be already
// in place via `make e2e-setup`.
func Setup(namespace string) (client.Client, string, func()) {
	c := NewClient()
	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
	err := c.Create(ctx, ns)
	gomega.Expect(client.IgnoreAlreadyExists(err)).NotTo(gomega.HaveOccurred())

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: ObjStoreSecret, Namespace: namespace},
		Type:       corev1.SecretTypeOpaque,
		Data:       map[string][]byte{ObjStoreSecretKey: []byte(objStoreConfig)},
	}
	err = c.Create(ctx, secret)
	gomega.Expect(client.IgnoreAlreadyExists(err)).NotTo(gomega.HaveOccurred())

	teardown := func() {
		_ = c.Delete(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})
	}
	return c, namespace, teardown
}

// ObjStoreConfig references the per-namespace object-storage secret Setup creates,
// for use in a component's ObjectStorageConfig field.
func ObjStoreConfig() v1alpha1.ObjectStorageConfig {
	return v1alpha1.ObjectStorageConfig{
		LocalObjectReference: corev1.LocalObjectReference{Name: ObjStoreSecret},
		Key:                  ObjStoreSecretKey,
	}
}

// ThanosVersion returns the Thanos image version the suites deploy, overridable via
// THANOS_VERSION (matches the monolith default).
func ThanosVersion() *string {
	v := os.Getenv("THANOS_VERSION")
	if v == "" {
		v = "v0.40.1"
	}
	return ptr.To(v)
}

// ReceiveName and QueryName are the fixed names the NewReceive/NewQuery fixtures use
// for the components they create. Feature suites reference them (e.g. to build the
// receive router match labels for a remote-write) without re-deriving the name.
const (
	ReceiveName = "example-receive"
	QueryName   = "example-query"
)

// NewReceive creates a minimal single-hashring ThanosReceive in namespace, waits for
// its ingester and router to become ready, and returns the parent name. The operator
// labels the generated ingester service as a store-api endpoint (so a ThanosQuery in
// the same namespace discovers it) and the router service as a remote-write endpoint
// (so a stateless ThanosRuler discovers it). External labels are namespaced so blocks
// from different suites' receives never collide in the shared bucket.
func NewReceive(c client.Client, namespace string) string {
	cr := &v1alpha1.ThanosReceive{
		ObjectMeta: metav1.ObjectMeta{Name: ReceiveName, Namespace: namespace},
		Spec: v1alpha1.ThanosReceiveSpec{
			StatefulSetFields: v1alpha1.StatefulSetFields{
				MinReadySeconds: ptr.To(int32(1)),
			},
			Ingester: v1alpha1.IngesterSpec{
				DefaultObjectStorageConfig: ObjStoreConfig(),
				Hashrings: []v1alpha1.IngesterHashringSpec{
					{
						Name: "default",
						StorageConfiguration: v1alpha1.StorageConfiguration{
							Size: resourceapi.MustParse("100Mi"),
						},
						CommonFields: v1alpha1.CommonFields{Version: ThanosVersion()},
					},
				},
			},
			Router: v1alpha1.RouterSpec{
				CommonFields:      v1alpha1.CommonFields{Version: ThanosVersion()},
				Replicas:          1,
				ReplicationFactor: 1,
				HashringPolicy:    ptr.To(v1alpha1.HashringPolicyStatic),
				ExternalLabels:    map[string]string{"receive": namespace},
			},
		},
	}
	gomega.Expect(c.Create(context.Background(), cr)).To(gomega.Succeed())

	ingester := controller.ReceiveIngesterNameFromParent(ReceiveName, "default")
	router := controller.ReceiveRouterNameFromParent(ReceiveName)
	gomega.Eventually(func() bool {
		return utils.VerifyStatefulSetReplicasRunning(c, 1, ingester, namespace)
	}, time.Minute*5, time.Second*2).Should(gomega.BeTrue())
	gomega.Eventually(func() bool {
		return utils.VerifyDeploymentReplicasRunning(c, 1, router, namespace)
	}, time.Minute*5, time.Second*2).Should(gomega.BeTrue())
	return ReceiveName
}

// NewQuery creates a ThanosQuery in namespace that discovers store-api endpoints,
// waits for it to become ready, and returns the parent name. Pair it with NewReceive
// so the query fans out to that receive's ingesters.
func NewQuery(c client.Client, namespace string) string {
	cr := &v1alpha1.ThanosQuery{
		ObjectMeta: metav1.ObjectMeta{Name: QueryName, Namespace: namespace},
		Spec: v1alpha1.ThanosQuerySpec{
			CommonFields: v1alpha1.CommonFields{Version: ThanosVersion()},
			Replicas:     1,
			ReplicaLabels: []string{
				"prometheus_replica",
				"replica",
				"rule_replica",
			},
			StoreLabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					manifests.DefaultStoreAPILabel: manifests.DefaultStoreAPIValue,
				},
			},
		},
	}
	gomega.Expect(c.Create(context.Background(), cr)).To(gomega.Succeed())

	gomega.Eventually(func() bool {
		return utils.VerifyDeploymentReplicasRunning(c, 1, controller.QueryNameFromParent(QueryName), namespace)
	}, time.Minute*3, time.Second*2).Should(gomega.BeTrue())
	return QueryName
}

// NewClient builds a controller-runtime client with the scheme the e2e suites need
// (v1alpha1 CRDs, apps, core, monitoring, rbac) from the ambient kubeconfig.
func NewClient() client.Client {
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		v1alpha1.AddToScheme, appsv1.AddToScheme, corev1.AddToScheme,
		monitoringv1.AddToScheme, rbacv1.AddToScheme,
	} {
		gomega.Expect(add(scheme)).To(gomega.Succeed())
	}
	c, err := client.New(config.GetConfigOrDie(), client.Options{Scheme: scheme})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	return c
}
