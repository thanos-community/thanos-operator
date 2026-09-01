package kuberesourcesync

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/controller"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Observable names of the artifacts the kube-resource-sync gate adds to the
// receive router. These are the injected sidecar/init container names, the
// hashring volume, and the extra Service port.
const (
	syncContainerName     = "kube-resource-sync"
	syncInitContainerName = "kube-resource-sync-init"
	hashringVolumeName    = "hashring-config"
	syncPortName          = "kube-resource-sync"
)

func createNamespace(ns string) {
	Expect(k8sClient.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: ns},
	})).Should(Succeed())
}

func hasContainer(cs []corev1.Container, name string) *corev1.Container {
	for i := range cs {
		if cs[i].Name == name {
			return &cs[i]
		}
	}
	return nil
}

var _ = Describe("KubeResourceSync feature gate", Ordered, func() {
	const ns = "krs-receive"
	const receiveName = "test-receive"

	var router string

	BeforeAll(func() {
		createNamespace(ns)
		router = controller.ReceiveRouterNameFromParent(receiveName)

		Expect(k8sClient.Create(ctx, &monitoringthanosiov1alpha1.ThanosReceive{
			ObjectMeta: metav1.ObjectMeta{Name: receiveName, Namespace: ns},
			Spec: monitoringthanosiov1alpha1.ThanosReceiveSpec{
				Router: monitoringthanosiov1alpha1.RouterSpec{
					ReplicationFactor: 1,
					Replicas:          1,
				},
				Ingester: monitoringthanosiov1alpha1.IngesterSpec{
					DefaultObjectStorageConfig: monitoringthanosiov1alpha1.ObjectStorageConfig{
						LocalObjectReference: corev1.LocalObjectReference{Name: "test-secret"},
						Key:                  "test-key",
					},
					Hashrings: []monitoringthanosiov1alpha1.IngesterHashringSpec{
						{
							Name: "test-hashring",
							StorageConfiguration: monitoringthanosiov1alpha1.StorageConfiguration{
								Size: resource.MustParse("100Mi"),
							},
							Replicas: 1,
						},
					},
				},
			},
		})).Should(Succeed())
	})

	It("adds the sync sidecar, init container, and hashring EmptyDir to the router deployment", func() {
		Eventually(func(g Gomega) {
			dep := &appsv1.Deployment{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: router, Namespace: ns}, dep)).To(Succeed())

			sidecar := hasContainer(dep.Spec.Template.Spec.Containers, syncContainerName)
			g.Expect(sidecar).NotTo(BeNil())
			g.Expect(sidecar.Image).To(Equal(kubeResourceSyncImage))

			init := hasContainer(dep.Spec.Template.Spec.InitContainers, syncInitContainerName)
			g.Expect(init).NotTo(BeNil())

			var vol *corev1.Volume
			for i := range dep.Spec.Template.Spec.Volumes {
				if dep.Spec.Template.Spec.Volumes[i].Name == hashringVolumeName {
					vol = &dep.Spec.Template.Spec.Volumes[i]
				}
			}
			g.Expect(vol).NotTo(BeNil())
			g.Expect(vol.EmptyDir).NotTo(BeNil())
		}).Should(Succeed())
	})

	It("creates a Role and RoleBinding granting the router ConfigMap read access", func() {
		Eventually(func(g Gomega) {
			role := &rbacv1.Role{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: router, Namespace: ns}, role)).To(Succeed())
			g.Expect(role.Rules).To(ContainElement(SatisfyAll(
				WithTransform(func(r rbacv1.PolicyRule) []string { return r.Resources }, ContainElement("configmaps")),
				WithTransform(func(r rbacv1.PolicyRule) []string { return r.Verbs }, ContainElements("get", "list", "watch")),
			)))

			rb := &rbacv1.RoleBinding{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: router, Namespace: ns}, rb)).To(Succeed())
			g.Expect(rb.RoleRef.Name).To(Equal(router))
			g.Expect(rb.Subjects).To(ContainElement(SatisfyAll(
				WithTransform(func(s rbacv1.Subject) string { return s.Kind }, Equal("ServiceAccount")),
				WithTransform(func(s rbacv1.Subject) string { return s.Name }, Equal(router)),
			)))
		}).Should(Succeed())
	})

	It("exposes the sync port on the router service", func() {
		Eventually(func(g Gomega) {
			svc := &corev1.Service{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: router, Namespace: ns}, svc)).To(Succeed())
			g.Expect(svc.Spec.Ports).To(ContainElement(
				WithTransform(func(p corev1.ServicePort) string { return p.Name }, Equal(syncPortName)),
			))
		}).Should(Succeed())
	})
})
