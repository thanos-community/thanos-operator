package pause

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/controller"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/compact"
	manifestquery "github.com/thanos-community/thanos-operator/internal/pkg/manifests/query"
	"github.com/thanos-community/thanos-operator/test/utils"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

const objstoreYAML = `type: S3
config:
  bucket: test
  endpoint: http://localhost:9000
  access_key: Cheesecake
  secret_key: supersecret
  http_config:
    insecure_skip_verify: false
`

// debugArg is the arg the mutation would add if a paused reconcile ran. The
// workloads are created without it, so its absence proves pause held.
const debugArg = "--log.level=debug"

const (
	ns           = "pause"
	queryName    = "test-query"
	receiveName  = "test-receive"
	hashringName = "test-hashring"
	storeName    = "test-store"
	rulerName    = "test-ruler"
	compactName  = "test-compact"
	shardName    = "test-shard"
)

// Names of the workloads the five reconcilers produce in this namespace.
var (
	queryWorkload    = manifestquery.Options{Options: manifests.Options{Owner: queryName}}.GetGeneratedResourceName()
	routerWorkload   = controller.ReceiveRouterNameFromParent(receiveName)
	ingesterWorkload = controller.ReceiveIngesterNameFromParent(receiveName, hashringName)
	storeWorkload    = controller.StoreNameFromParent(storeName, nil)
	rulerWorkload    = controller.RulerNameFromParent(rulerName)
	compactWorkload  = compact.Options{
		Options:   manifests.Options{Owner: compactName},
		ShardName: ptr.To(shardName),
	}.GetGeneratedResourceName()
)

var _ = Describe("Paused reconciliation", func() {
	BeforeEach(func() {
		Expect(k8sClient.Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: ns},
		})).Should(Succeed())
		Expect(k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "thanos-objstore", Namespace: ns},
			StringData: map[string]string{"thanos.yaml": objstoreYAML},
		})).Should(Succeed())

		Expect(k8sClient.Create(ctx, &monitoringthanosiov1alpha1.ThanosQuery{
			ObjectMeta: metav1.ObjectMeta{Name: queryName, Namespace: ns},
			Spec: monitoringthanosiov1alpha1.ThanosQuerySpec{
				ReplicaLabels: []string{"replica"},
				Replicas:      1,
			},
		})).Should(Succeed())

		Expect(k8sClient.Create(ctx, &monitoringthanosiov1alpha1.ThanosReceive{
			ObjectMeta: metav1.ObjectMeta{Name: receiveName, Namespace: ns},
			Spec: monitoringthanosiov1alpha1.ThanosReceiveSpec{
				Router: monitoringthanosiov1alpha1.RouterSpec{
					ReplicationFactor: 1,
					Replicas:          1,
				},
				Ingester: monitoringthanosiov1alpha1.IngesterSpec{
					DefaultObjectStorageConfig: monitoringthanosiov1alpha1.ObjectStorageConfig{
						LocalObjectReference: corev1.LocalObjectReference{Name: "thanos-objstore"},
						Key:                  "thanos.yaml",
					},
					Hashrings: []monitoringthanosiov1alpha1.IngesterHashringSpec{
						{
							Name: hashringName,
							StorageConfiguration: monitoringthanosiov1alpha1.StorageConfiguration{
								Size: resource.MustParse("100Mi"),
							},
							Replicas: 1,
						},
					},
				},
			},
		})).Should(Succeed())

		Expect(k8sClient.Create(ctx, &monitoringthanosiov1alpha1.ThanosStore{
			ObjectMeta: metav1.ObjectMeta{Name: storeName, Namespace: ns},
			Spec: monitoringthanosiov1alpha1.ThanosStoreSpec{
				Replicas: 1,
				StorageConfiguration: monitoringthanosiov1alpha1.StorageConfiguration{
					Size: resource.MustParse("1Gi"),
				},
				ObjectStorageConfig: monitoringthanosiov1alpha1.ObjectStorageConfig{
					LocalObjectReference: corev1.LocalObjectReference{Name: "thanos-objstore"},
					Key:                  "thanos.yaml",
				},
			},
		})).Should(Succeed())

		Expect(k8sClient.Create(ctx, &monitoringthanosiov1alpha1.ThanosRuler{
			ObjectMeta: metav1.ObjectMeta{Name: rulerName, Namespace: ns},
			Spec: monitoringthanosiov1alpha1.ThanosRulerSpec{
				Replicas: 1,
				StorageConfiguration: monitoringthanosiov1alpha1.StorageConfiguration{
					Size: resource.MustParse("1Gi"),
				},
				RulerMode: monitoringthanosiov1alpha1.RulerMode{
					Type: "Stateful",
					Stateful: &monitoringthanosiov1alpha1.StatefulSpec{
						ObjectStorageConfig: monitoringthanosiov1alpha1.ObjectStorageConfig{
							LocalObjectReference: corev1.LocalObjectReference{Name: "thanos-objstore"},
							Key:                  "thanos.yaml",
						},
					},
				},
				RuleConfigSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue,
					},
				},
				AlertmanagerURL: "http://alertmanager.com:9093",
			},
		})).Should(Succeed())

		Expect(k8sClient.Create(ctx, &monitoringthanosiov1alpha1.ThanosCompact{
			ObjectMeta: metav1.ObjectMeta{Name: compactName, Namespace: ns},
			Spec: monitoringthanosiov1alpha1.ThanosCompactSpec{
				ShardingConfig: []monitoringthanosiov1alpha1.ShardingConfig{
					{
						ShardName: shardName,
						ExternalLabelSharding: []monitoringthanosiov1alpha1.ExternalLabelShardingConfig{
							{Label: "tenant_id", Value: "someone"},
						},
					},
				},
				StorageConfiguration: monitoringthanosiov1alpha1.StorageConfiguration{
					Size: resource.MustParse("1Gi"),
				},
				ObjectStorageConfig: monitoringthanosiov1alpha1.ObjectStorageConfig{
					LocalObjectReference: corev1.LocalObjectReference{Name: "thanos-objstore"},
					Key:                  "thanos.yaml",
				},
			},
		})).Should(Succeed())

		// Every workload must exist (and, being freshly created, must not carry
		// the debug arg) before we pause, so the Consistently below has a real
		// baseline to hold against.
		Eventually(func(g Gomega) {
			g.Expect(utils.VerifyDeploymentExists(k8sClient, queryWorkload, ns)).To(BeTrue())
			g.Expect(utils.VerifyDeploymentExists(k8sClient, routerWorkload, ns)).To(BeTrue())
			g.Expect(utils.VerifyStatefulSetExists(k8sClient, ingesterWorkload, ns)).To(BeTrue())
			g.Expect(utils.VerifyStatefulSetExists(k8sClient, storeWorkload, ns)).To(BeTrue())
			g.Expect(utils.VerifyStatefulSetExists(k8sClient, rulerWorkload, ns)).To(BeTrue())
			g.Expect(utils.VerifyStatefulSetExists(k8sClient, compactWorkload, ns)).To(BeTrue())
		}).Should(Succeed())
	})

	It("does not apply spec changes to any component while paused", func() {
		By("pausing every component and mutating its log level in the same update")
		pauseAndSetDebug()

		By("verifying no workload ever reconciles the change (one shared window)")
		Consistently(func(g Gomega) {
			g.Expect(utils.VerifyDeploymentArgs(k8sClient, queryWorkload, ns, 0, debugArg)).To(BeFalse())
			g.Expect(utils.VerifyDeploymentArgs(k8sClient, routerWorkload, ns, 0, debugArg)).To(BeFalse())
			g.Expect(utils.VerifyStatefulSetArgs(k8sClient, ingesterWorkload, ns, 0, debugArg)).To(BeFalse())
			g.Expect(utils.VerifyStatefulSetArgs(k8sClient, storeWorkload, ns, 0, debugArg)).To(BeFalse())
			g.Expect(utils.VerifyStatefulSetArgs(k8sClient, rulerWorkload, ns, 0, debugArg)).To(BeFalse())
			g.Expect(utils.VerifyStatefulSetArgs(k8sClient, compactWorkload, ns, 0, debugArg)).To(BeFalse())
		}).Should(Succeed())
	})
})

// pauseAndSetDebug reads each CR fresh, sets Paused and a debug log level in one
// update, so any version that carries the mutation also carries the pause.
func pauseAndSetDebug() {
	query := &monitoringthanosiov1alpha1.ThanosQuery{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: queryName, Namespace: ns}, query)).Should(Succeed())
	query.Spec.Paused = ptr.To(true)
	query.Spec.CommonFields.LogLevel = ptr.To("debug")
	Expect(k8sClient.Update(ctx, query)).Should(Succeed())

	receive := &monitoringthanosiov1alpha1.ThanosReceive{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: receiveName, Namespace: ns}, receive)).Should(Succeed())
	receive.Spec.Paused = ptr.To(true)
	receive.Spec.Router.CommonFields.LogLevel = ptr.To("debug")
	receive.Spec.Ingester.Hashrings[0].CommonFields.LogLevel = ptr.To("debug")
	Expect(k8sClient.Update(ctx, receive)).Should(Succeed())

	store := &monitoringthanosiov1alpha1.ThanosStore{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: storeName, Namespace: ns}, store)).Should(Succeed())
	store.Spec.Paused = ptr.To(true)
	store.Spec.CommonFields.LogLevel = ptr.To("debug")
	Expect(k8sClient.Update(ctx, store)).Should(Succeed())

	ruler := &monitoringthanosiov1alpha1.ThanosRuler{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: rulerName, Namespace: ns}, ruler)).Should(Succeed())
	ruler.Spec.Paused = ptr.To(true)
	ruler.Spec.CommonFields.LogLevel = ptr.To("debug")
	Expect(k8sClient.Update(ctx, ruler)).Should(Succeed())

	comp := &monitoringthanosiov1alpha1.ThanosCompact{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: compactName, Namespace: ns}, comp)).Should(Succeed())
	comp.Spec.Paused = ptr.To(true)
	comp.Spec.CommonFields.LogLevel = ptr.To("debug")
	Expect(k8sClient.Update(ctx, comp)).Should(Succeed())
}
