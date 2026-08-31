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

package query

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/controller"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	manifestsquery "github.com/thanos-community/thanos-operator/internal/pkg/manifests/query"
	"github.com/thanos-community/thanos-operator/test/e2e/suite"
	"github.com/thanos-community/thanos-operator/test/utils"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Thanos Query", Ordered, func() {
	const queryName = "example-query"

	// Fake StoreAPI services representing components the query should discover without
	// actually running them. Each carries the store-api discovery labels and a grpc
	// port so the query controller lists it as an endpoint; the endpoint-group label on
	// the store makes it render as an HA endpoint group. No pods back these services --
	// the query bakes the flags into its pod template and stays ready because the
	// endpoints are non-strict (dynamically discovered).
	const (
		fakeReceiveIngester = "fake-receive-ingester"
		fakeRuler           = "fake-ruler"
		fakeStoreHA         = "fake-store-ha"
	)

	It("should discover fake store-api services and bring up the query", func() {
		By("planting fake store-api services for a receive ingester, a ruler and an HA store")
		Expect(createStoreAPIService(fakeReceiveIngester, false)).To(Succeed())
		Expect(createStoreAPIService(fakeRuler, false)).To(Succeed())
		Expect(createStoreAPIService(fakeStoreHA, true)).To(Succeed())

		By("creating a ThanosQuery without a query frontend")
		cr := &v1alpha1.ThanosQuery{
			ObjectMeta: metav1.ObjectMeta{
				Name:      queryName,
				Namespace: namespace,
			},
			Spec: v1alpha1.ThanosQuerySpec{
				CommonFields: v1alpha1.CommonFields{Version: suite.ThanosVersion()},
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
		Expect(c.Create(context.Background(), cr)).To(Succeed())

		By("waiting for the query deployment to become ready")
		Eventually(func() bool {
			return utils.VerifyDeploymentReplicasRunning(c, 1, controller.QueryNameFromParent(queryName), namespace)
		}, time.Minute*3, time.Second*2).Should(BeTrue())
	})

	It("should render an endpoint flag for each discovered store-api service", func() {
		queryDeployment := controller.QueryNameFromParent(queryName)
		Eventually(func() bool {
			return utils.VerifyDeploymentArgs(c, queryDeployment, namespace, 0,
				fmt.Sprintf("--endpoint=dnssrv+_grpc._tcp.%s.%s.svc", fakeReceiveIngester, namespace),
				fmt.Sprintf("--endpoint=dnssrv+_grpc._tcp.%s.%s.svc", fakeRuler, namespace),
				fmt.Sprintf("--endpoint-group=dnssrv+_grpc._tcp.%s.%s.svc", fakeStoreHA, namespace),
			)
		}, time.Minute*2, time.Second*2).Should(BeTrue())
	})

	It("should bring up the query frontend when patched onto the query", func() {
		By("patching the ThanosQuery to add a query frontend")
		tenSeconds := v1alpha1.Duration("10s")
		cr := &v1alpha1.ThanosQuery{}
		Expect(c.Get(context.Background(), client.ObjectKey{Name: queryName, Namespace: namespace}, cr)).To(Succeed())
		cr.Spec.QueryFrontend = &v1alpha1.QueryFrontendSpec{
			CommonFields:         v1alpha1.CommonFields{Version: suite.ThanosVersion()},
			Replicas:             1,
			CompressResponses:    true,
			LogQueriesLongerThan: &tenSeconds,
			QueryLabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					manifests.DefaultQueryAPILabel: manifests.DefaultQueryAPIValue,
				},
			},
		}
		Expect(c.Update(context.Background(), cr)).To(Succeed())

		By("waiting for the query frontend deployment to become ready")
		Eventually(func() bool {
			return utils.VerifyDeploymentReplicasRunning(c, 1, controller.QueryFrontendNameFromParent(queryName), namespace)
		}, time.Minute*3, time.Second*2).Should(BeTrue())
	})

	It("should wire the query frontend downstream url to the query service", func() {
		qfeDeployment := controller.QueryFrontendNameFromParent(queryName)
		queryService := controller.QueryNameFromParent(queryName)
		Eventually(func() bool {
			return utils.VerifyDeploymentArgs(c, qfeDeployment, namespace, 0,
				fmt.Sprintf("--query-frontend.downstream-url=http://%s.%s.svc:9090", queryService, namespace),
			)
		}, time.Minute*2, time.Second*2).Should(BeTrue())
	})
})

// createStoreAPIService plants a Service carrying the store-api discovery labels and a
// grpc port so the query controller renders it as an endpoint. When ha is true it also
// carries the endpoint-group label so the query renders it as an HA endpoint group. No
// pods back the service; it exists only to be discovered.
func createStoreAPIService(name string, ha bool) error {
	labels := map[string]string{
		manifests.PartOfLabel:          manifests.DefaultPartOfLabel,
		manifests.DefaultStoreAPILabel: manifests.DefaultStoreAPIValue,
	}
	if ha {
		labels[string(manifests.GroupLabel)] = "true"
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: corev1.ClusterIPNone,
			Ports: []corev1.ServicePort{
				{
					Name:       manifestsquery.GRPCPortName,
					Port:       manifestsquery.GRPCPort,
					TargetPort: intstr.FromInt32(manifestsquery.GRPCPort),
				},
			},
			// No selector so no pods back the service; the query still renders the flag.
			Selector: map[string]string{"e2e-query.fake": name},
		},
	}
	return client.IgnoreAlreadyExists(c.Create(context.Background(), svc))
}
