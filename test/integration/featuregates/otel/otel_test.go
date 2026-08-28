package otel

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/controller"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/compact"
	manifestquery "github.com/thanos-community/thanos-operator/internal/pkg/manifests/query"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

// otelInjectAnnotation is the pod-template annotation the OpenTelemetry operator
// watches to inject a collector sidecar. The operator sets it on the workload's
// pod template, not on the workload object itself.
const otelInjectAnnotation = "sidecar.opentelemetry.io/inject"

const objstoreYAML = `type: S3
config:
  bucket: test
  endpoint: http://localhost:9000
  access_key: Cheesecake
  secret_key: supersecret
  http_config:
    insecure_skip_verify: false
`

func createNamespace(ns string) {
	Expect(k8sClient.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: ns},
	})).Should(Succeed())
}

func createObjstoreSecret(ns string) {
	Expect(k8sClient.Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "thanos-objstore", Namespace: ns},
		StringData: map[string]string{"thanos.yaml": objstoreYAML},
	})).Should(Succeed())
}

// hasTracingArg reports whether the container carries the injected tracing arg.
// The arg value is a multiline YAML blob, so match on the flag prefix rather
// than the exact string.
func hasTracingArg(args []string) bool {
	for _, a := range args {
		if strings.HasPrefix(a, "--tracing.config=") {
			return true
		}
	}
	return false
}

// deploymentHasOtel checks the injected annotation lands on the pod template and
// the tracing arg lands on the main container of a Deployment.
func deploymentHasOtel(name, ns string) bool {
	dep := &appsv1.Deployment{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, dep); err != nil {
		return false
	}
	if dep.Spec.Template.Annotations[otelInjectAnnotation] != "true" {
		return false
	}
	return len(dep.Spec.Template.Spec.Containers) > 0 &&
		hasTracingArg(dep.Spec.Template.Spec.Containers[0].Args)
}

// statefulSetHasOtel is the StatefulSet counterpart of deploymentHasOtel.
func statefulSetHasOtel(name, ns string) bool {
	sts := &appsv1.StatefulSet{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, sts); err != nil {
		return false
	}
	if sts.Spec.Template.Annotations[otelInjectAnnotation] != "true" {
		return false
	}
	return len(sts.Spec.Template.Spec.Containers) > 0 &&
		hasTracingArg(sts.Spec.Template.Spec.Containers[0].Args)
}

var _ = Describe("OtelSidecar feature gate", func() {
	It("injects the sidecar into ThanosQuery", func() {
		const ns = "otel-query"
		createNamespace(ns)

		queryName := "test-query"
		name := manifestquery.Options{Options: manifests.Options{Owner: queryName}}.GetGeneratedResourceName()

		Expect(k8sClient.Create(ctx, &monitoringthanosiov1alpha1.ThanosQuery{
			ObjectMeta: metav1.ObjectMeta{Name: queryName, Namespace: ns},
			Spec: monitoringthanosiov1alpha1.ThanosQuerySpec{
				ReplicaLabels: []string{"replica"},
				Replicas:      1,
			},
		})).Should(Succeed())

		Eventually(func() bool {
			return deploymentHasOtel(name, ns)
		}).Should(BeTrue())
	})

	It("injects the sidecar into ThanosReceive ingester and router", func() {
		const ns = "otel-receive"
		createNamespace(ns)

		receiveName := "test-receive"
		hashringName := "test-hashring"
		router := controller.ReceiveRouterNameFromParent(receiveName)
		ingester := controller.ReceiveIngesterNameFromParent(receiveName, hashringName)

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

		Eventually(func() bool {
			return deploymentHasOtel(router, ns)
		}).Should(BeTrue())
		Eventually(func() bool {
			return statefulSetHasOtel(ingester, ns)
		}).Should(BeTrue())
	})

	It("injects the sidecar into ThanosStore", func() {
		const ns = "otel-store"
		createNamespace(ns)
		createObjstoreSecret(ns)

		storeName := "test-store"
		firstShard := controller.StoreNameFromParent(storeName, ptr.To(int32(0)))

		Expect(k8sClient.Create(ctx, &monitoringthanosiov1alpha1.ThanosStore{
			ObjectMeta: metav1.ObjectMeta{Name: storeName, Namespace: ns},
			Spec: monitoringthanosiov1alpha1.ThanosStoreSpec{
				Replicas: 1,
				ShardingStrategy: monitoringthanosiov1alpha1.ShardingStrategy{
					Type:   monitoringthanosiov1alpha1.Block,
					Shards: 3,
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

		Eventually(func() bool {
			return statefulSetHasOtel(firstShard, ns)
		}).Should(BeTrue())
	})

	It("injects the sidecar into ThanosRuler", func() {
		const ns = "otel-ruler"
		createNamespace(ns)
		createObjstoreSecret(ns)

		// The ruler discovers query endpoints via same-namespace SD, so a query
		// must exist alongside it or reconciliation stops before the workload.
		Expect(k8sClient.Create(ctx, &monitoringthanosiov1alpha1.ThanosQuery{
			ObjectMeta: metav1.ObjectMeta{Name: "test-query", Namespace: ns},
			Spec: monitoringthanosiov1alpha1.ThanosQuerySpec{
				ReplicaLabels: []string{"replica"},
				Replicas:      1,
			},
		})).Should(Succeed())

		rulerName := "test-ruler"
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

		Eventually(func() bool {
			return statefulSetHasOtel(controller.RulerNameFromParent(rulerName), ns)
		}).Should(BeTrue())
	})

	It("injects the sidecar into ThanosCompact", func() {
		const ns = "otel-compact"
		createNamespace(ns)
		createObjstoreSecret(ns)

		compactName := "test-compact"
		shardName := "test-shard"
		shard := compact.Options{
			Options:   manifests.Options{Owner: compactName},
			ShardName: ptr.To(shardName),
		}.GetGeneratedResourceName()

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

		Eventually(func() bool {
			return statefulSetHasOtel(shard, ns)
		}).Should(BeTrue())
	})
})
