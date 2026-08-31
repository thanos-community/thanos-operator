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

package capnproto

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/controller"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/receive"
	"github.com/thanos-community/thanos-operator/test/e2e/suite"
	"github.com/thanos-community/thanos-operator/test/utils"

	resourceapi "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const hashringName = "default"

var _ = Describe("Thanos Receive with capnproto replication protocol", Ordered, func() {
	capnprotoReceiveName := "capnproto-receive"
	capnprotoRouterName := controller.ReceiveRouterNameFromParent(capnprotoReceiveName)
	capnprotoIngesterName := controller.ReceiveIngesterNameFromParent(capnprotoReceiveName, hashringName)

	It("should bring up components with capnproto configuration", func() {
		cr := &v1alpha1.ThanosReceive{
			ObjectMeta: metav1.ObjectMeta{
				Name:      capnprotoReceiveName,
				Namespace: namespace,
			},
			Spec: v1alpha1.ThanosReceiveSpec{
				StatefulSetFields: v1alpha1.StatefulSetFields{
					MinReadySeconds: ptr.To(int32(1)),
				},
				Ingester: v1alpha1.IngesterSpec{
					DefaultObjectStorageConfig: suite.ObjStoreConfig(),
					Hashrings: []v1alpha1.IngesterHashringSpec{
						{
							Name: hashringName,
							StorageConfiguration: v1alpha1.StorageConfiguration{
								Size: resourceapi.MustParse("100Mi"),
							},
							CommonFields: v1alpha1.CommonFields{
								Version: suite.ThanosVersion(),
							},
						},
					},
				},
				Router: v1alpha1.RouterSpec{
					CommonFields: v1alpha1.CommonFields{
						Version: suite.ThanosVersion(),
					},
					Replicas:            1,
					ReplicationFactor:   1,
					ReplicationProtocol: ptr.To(v1alpha1.ReplicationProtocolCapnProto),
					HashringPolicy:      ptr.To(v1alpha1.HashringPolicyStatic),
					ExternalLabels: map[string]string{
						"receive": "capnproto",
					},
				},
			},
		}
		err := c.Create(context.Background(), cr, &client.CreateOptions{})
		Expect(err).To(BeNil())

		Eventually(func() bool {
			return utils.VerifyStatefulSetReplicasRunning(c, 1, capnprotoIngesterName, namespace)
		}, time.Minute*5, time.Second*2).Should(BeTrue())

		Eventually(func() bool {
			return utils.VerifyDeploymentReplicasRunning(c, 1, capnprotoRouterName, namespace)
		}, time.Minute*5, time.Second*2).Should(BeTrue())
	})

	It("should create ConfigMap with capnproto_address fields", func() {
		//nolint:lll
		expect := fmt.Sprintf(`[
    {
        "hashring": "%s",
        "endpoints": [
            {
                "address": "%s-0.%s.%s.svc:10901",
				"capnproto_address": "%s-0.%s.%s.svc:19391",
                "az": ""
            }
        ],
        "algorithm": "ketama",
		"external_labels": {}
    }
]`, hashringName, capnprotoIngesterName, capnprotoIngesterName, namespace, capnprotoIngesterName, capnprotoIngesterName, namespace)

		Eventually(func() bool {
			return utils.VerifyConfigMapContents(c, capnprotoRouterName, namespace, receive.HashringConfigKey, expect)
		}, time.Minute*5, time.Second*2).Should(BeTrue())
	})

	It("should have capnproto arguments in router deployment", func() {
		Eventually(func() bool {
			return utils.VerifyDeploymentArgs(c,
				capnprotoRouterName,
				namespace,
				0,
				"--receive.replication-protocol=capnproto",
			)
		}, time.Minute*1, time.Second*2).Should(BeTrue())
	})

	It("should have capnproto arguments in ingester statefulset", func() {
		Eventually(func() bool {
			return utils.VerifyStatefulSetArgs(c,
				capnprotoIngesterName,
				namespace,
				0,
				"--receive.capnproto-address=0.0.0.0:19391",
			)
		}, time.Minute*1, time.Second*2).Should(BeTrue())
	})

	It("should accept metrics over remote write with capnproto protocol", func() {
		matchLabels := map[string]string{
			manifests.ComponentLabel: receive.RouterComponentName,
			manifests.OwnerLabel:     capnprotoReceiveName,
		}
		Eventually(func() error {
			return utils.DoRemoteWriteRequest(c, utils.DefaultRemoteWriteRequest(), namespace, matchLabels, nil, receive.RemoteWritePort)
		}, time.Minute*2, time.Second*1).Should(Succeed())
	})
})
