package namespace

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	"github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/test/e2e/suite"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestNamespaceIsolation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Namespace isolation E2E Suite")
}

var _ = Describe("Operators in separate namespaces", func() {
	It("keeps feature configuration local and leaves unwatched resources alone", func() {
		ctx := context.Background()
		const monitored, plain = "e2e-ns-monitored", "e2e-ns-plain"
		c := suite.Setup(monitored, featuregate.ServiceMonitor)
		suite.Setup(plain)
		outside := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "e2e-ns-unwatched"}}
		Expect(c.Create(ctx, outside)).To(Succeed())
		DeferCleanup(func() { Expect(c.Delete(ctx, outside)).To(Succeed()) })

		suite.NewQuery(c, monitored)
		suite.NewQuery(c, plain)
		unwatched := &v1alpha1.ThanosQuery{
			ObjectMeta: metav1.ObjectMeta{Name: suite.QueryName, Namespace: outside.Name},
			Spec:       v1alpha1.ThanosQuerySpec{Replicas: 1},
		}
		Expect(c.Create(ctx, unwatched)).To(Succeed())

		Eventually(func() (int, error) {
			monitors := &monitoringv1.ServiceMonitorList{}
			err := c.List(ctx, monitors, client.InNamespace(monitored))
			return len(monitors.Items), err
		}, time.Minute, time.Second).Should(Equal(1))

		Consistently(func(g Gomega) {
			monitors := &monitoringv1.ServiceMonitorList{}
			g.Expect(c.List(ctx, monitors, client.InNamespace(plain))).To(Succeed())
			g.Expect(monitors.Items).To(BeEmpty())
			deployments := &appsv1.DeploymentList{}
			g.Expect(c.List(ctx, deployments, client.InNamespace(outside.Name))).To(Succeed())
			g.Expect(deployments.Items).To(BeEmpty())
			current := &v1alpha1.ThanosQuery{}
			g.Expect(c.Get(ctx, client.ObjectKeyFromObject(unwatched), current)).To(Succeed())
			g.Expect(current.Status).To(Equal(unwatched.Status))
		}, 10*time.Second, time.Second).Should(Succeed())
	})
})
