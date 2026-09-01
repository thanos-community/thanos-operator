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

package kuberesourcesync

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/controller"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/receive"
	"github.com/thanos-community/thanos-operator/test/e2e/suite"
	"github.com/thanos-community/thanos-operator/test/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	resourceapi "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Observable artifacts the kube-resource-sync gate adds to the receive router: the
// injected sidecar/init container names, the hashring volume, and the extra Service
// port. These mirror the constants in the integration suite.
const (
	hashringName          = "default"
	syncContainerName     = "kube-resource-sync"
	syncInitContainerName = "kube-resource-sync-init"
	hashringVolumeName    = "hashring-config"
	syncPortName          = "kube-resource-sync"
	// kubeResourceSyncImage is the operator's default sidecar image, used when
	// e2e-setup deploys the operator without an image override.
	kubeResourceSyncImage = "quay.io/philipgough/kube-resource-sync:0.1.0"
)

func containerByName(cs []corev1.Container, name string) *corev1.Container {
	for i := range cs {
		if cs[i].Name == name {
			return &cs[i]
		}
	}
	return nil
}

var _ = Describe("Thanos Receive with kube-resource-sync", Ordered, func() {
	const receiveName = "krs-receive"
	routerName := controller.ReceiveRouterNameFromParent(receiveName)
	ingesterName := controller.ReceiveIngesterNameFromParent(receiveName, hashringName)

	It("should bring up the receive with the sync sidecar wired into the router", func() {
		cr := &v1alpha1.ThanosReceive{
			ObjectMeta: metav1.ObjectMeta{
				Name:      receiveName,
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
					Replicas:          1,
					ReplicationFactor: 1,
					HashringPolicy:    ptr.To(v1alpha1.HashringPolicyStatic),
					ExternalLabels:    map[string]string{"receive": namespace},
				},
			},
		}
		Expect(c.Create(context.Background(), cr)).To(Succeed())

		// The ingester must be up so the router can build a valid hashring.
		Eventually(func() bool {
			return utils.VerifyStatefulSetReplicasRunning(c, 1, ingesterName, namespace)
		}, time.Minute*5, time.Second*2).Should(BeTrue())

		// The router only becomes ready if the init container populated the hashring
		// EmptyDir from the ConfigMap: without a hashrings file thanos receive fails to
		// start. So a ready router already proves the sync init path works end to end.
		Eventually(func() bool {
			return utils.VerifyDeploymentReplicasRunning(c, 1, routerName, namespace)
		}, time.Minute*5, time.Second*2).Should(BeTrue())
	})

	It("should inject the sync sidecar, init container, and hashring EmptyDir", func() {
		dep := &appsv1.Deployment{}
		Expect(c.Get(context.Background(), client.ObjectKey{Name: routerName, Namespace: namespace}, dep)).To(Succeed())

		sidecar := containerByName(dep.Spec.Template.Spec.Containers, syncContainerName)
		Expect(sidecar).NotTo(BeNil())
		Expect(sidecar.Image).To(Equal(kubeResourceSyncImage))
		Expect(sidecar.Args).To(ConsistOf(
			"--resource-type=configmap",
			"--resource-name="+routerName,
			"--namespace="+namespace,
			"--write-path=/var/lib/thanos-receive/hashrings.json",
			"--resource-key=hashrings.json",
		))

		// The init container shares the sidecar's config but runs once in init mode to
		// pre-populate the EmptyDir before the router starts.
		init := containerByName(dep.Spec.Template.Spec.InitContainers, syncInitContainerName)
		Expect(init).NotTo(BeNil())
		Expect(init.Args).To(ContainElement("--init-mode"))

		var vol *corev1.Volume
		for i := range dep.Spec.Template.Spec.Volumes {
			if dep.Spec.Template.Spec.Volumes[i].Name == hashringVolumeName {
				vol = &dep.Spec.Template.Spec.Volumes[i]
			}
		}
		Expect(vol).NotTo(BeNil())
		Expect(vol.EmptyDir).NotTo(BeNil())
		Expect(vol.ConfigMap).To(BeNil())
	})

	It("should create a Role and RoleBinding granting the router ConfigMap read access", func() {
		role := &rbacv1.Role{}
		Expect(c.Get(context.Background(), client.ObjectKey{Name: routerName, Namespace: namespace}, role)).To(Succeed())
		Expect(role.Rules).To(ContainElement(SatisfyAll(
			WithTransform(func(r rbacv1.PolicyRule) []string { return r.Resources }, ContainElement("configmaps")),
			WithTransform(func(r rbacv1.PolicyRule) []string { return r.Verbs }, ContainElements("get", "list", "watch")),
		)))

		rb := &rbacv1.RoleBinding{}
		Expect(c.Get(context.Background(), client.ObjectKey{Name: routerName, Namespace: namespace}, rb)).To(Succeed())
		Expect(rb.RoleRef.Name).To(Equal(routerName))
		Expect(rb.Subjects).To(ContainElement(SatisfyAll(
			WithTransform(func(s rbacv1.Subject) string { return s.Kind }, Equal("ServiceAccount")),
			WithTransform(func(s rbacv1.Subject) string { return s.Name }, Equal(routerName)),
		)))
	})

	It("should expose the sync metrics port on the router service", func() {
		svc := &corev1.Service{}
		Expect(c.Get(context.Background(), client.ObjectKey{Name: routerName, Namespace: namespace}, svc)).To(Succeed())
		Expect(svc.Spec.Ports).To(ContainElement(
			WithTransform(func(p corev1.ServicePort) string { return p.Name }, Equal(syncPortName)),
		))
	})

	It("should route remote-write through the synced hashring", func() {
		// A successful write proves the hashrings file the sidecar synced into the
		// EmptyDir is valid and the router can route to the ingester using it.
		matchLabels := map[string]string{
			manifests.ComponentLabel: receive.RouterComponentName,
			manifests.OwnerLabel:     receiveName,
		}
		Eventually(func() error {
			return utils.DoRemoteWriteRequest(c, utils.DefaultRemoteWriteRequest(), namespace, matchLabels, nil, receive.RemoteWritePort)
		}, time.Minute*2, time.Second*1).Should(Succeed())
	})
})
