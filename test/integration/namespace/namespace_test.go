package namespace

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/controller"
	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/internal/pkg/metrics"
	"github.com/thanos-community/thanos-operator/test/integration/suite"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

const queryName = "shared-name"

func TestNamespaceIsolation(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	env, err := suite.Start("",
		filepath.Join(root, "config", "crd", "bases"),
		filepath.Join(root, "test", "integration", "configs", "service-monitor.yaml"),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, env.Stop()) })
	ctx := context.Background()
	namespaces := []string{"monitored", "plain", "unwatched"}
	for _, namespace := range namespaces {
		require.NoError(t, env.Client.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}))
		require.NoError(t, env.Client.Create(ctx, newQuery(namespace)))
		require.NoError(t, env.Client.Create(ctx, &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: "store-" + namespace, Namespace: namespace,
				Labels: map[string]string{manifests.DefaultStoreAPILabel: manifests.DefaultStoreAPIValue, manifests.PartOfLabel: manifests.DefaultPartOfLabel},
			},
			Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{{Name: "grpc", Port: 10901}}},
		}))
	}

	t.Run("scoped managers", func(t *testing.T) {
		startManager(t, env, namespaces[0], featuregate.Config{
			ServiceMonitor: &featuregate.ServiceMonitorConfig{FeatureConfig: featuregate.FeatureConfig{Enabled: true}},
		})
		startManager(t, env, namespaces[1], featuregate.Config{})
		for i, namespace := range namespaces[:2] {
			key := client.ObjectKey{Name: controller.QueryNameFromParent(queryName), Namespace: namespace}
			// Verify existing namespace-local discovery still works with a scoped cache.
			require.Eventually(t, func() bool {
				deployment := &appsv1.Deployment{}
				if env.Client.Get(ctx, key, deployment) != nil {
					return false
				}
				args := deployment.Spec.Template.Spec.Containers[0].Args
				for _, candidate := range namespaces {
					endpoint := fmt.Sprintf("--endpoint=dnssrv+_grpc._tcp.store-%s.%s.svc", candidate, candidate)
					if slices.Contains(args, endpoint) != (candidate == namespace) {
						return false
					}
				}
				return true
			}, time.Minute, 100*time.Millisecond, "discovery must remain in %s", namespace)

			// The status controller's unqualified Lists must obey the same scope.
			ready := int32(i + 1)
			deployment := &appsv1.Deployment{}
			require.NoError(t, env.Client.Get(ctx, key, deployment))
			deployment.Status = appsv1.DeploymentStatus{Replicas: ready, ReadyReplicas: ready, AvailableReplicas: ready}
			require.NoError(t, env.Client.Status().Update(ctx, deployment))
			require.Eventually(t, func() bool {
				query := &v1alpha1.ThanosQuery{}
				return env.Client.Get(ctx, client.ObjectKey{Name: queryName, Namespace: namespace}, query) == nil && query.Status.Querier.AvailableReplicas == ready
			}, time.Minute, 100*time.Millisecond)
		}

		monitorKey := client.ObjectKey{Name: controller.QueryNameFromParent(queryName), Namespace: namespaces[0]}
		require.Eventually(t, func() bool {
			return env.Client.Get(ctx, monitorKey, &monitoringv1.ServiceMonitor{}) == nil
		}, time.Minute, 100*time.Millisecond)

		outside := &v1alpha1.ThanosQuery{}
		outsideKey := client.ObjectKey{Name: queryName, Namespace: namespaces[2]}
		require.NoError(t, env.Client.Get(ctx, outsideKey, outside))
		require.Never(t, func() bool {
			plainMonitor := client.ObjectKey{Name: monitorKey.Name, Namespace: namespaces[1]}
			err := env.Client.Get(ctx, plainMonitor, &monitoringv1.ServiceMonitor{})
			if !apierrors.IsNotFound(err) {
				return true
			}
			deployments := &appsv1.DeploymentList{}
			if env.Client.List(ctx, deployments, client.InNamespace(namespaces[2])) != nil || len(deployments.Items) != 0 {
				return true
			}
			current := &v1alpha1.ThanosQuery{}
			return env.Client.Get(ctx, outsideKey, current) != nil || current.ResourceVersion != outside.ResourceVersion
		}, 3*time.Second, 100*time.Millisecond, "disabled features and unwatched resources must stay untouched")
	})

	t.Run("cluster-wide default", func(t *testing.T) {
		startManager(t, env, "", featuregate.Config{})
		for _, namespace := range namespaces {
			query := &v1alpha1.ThanosQuery{}
			key := client.ObjectKey{Name: queryName, Namespace: namespace}
			require.NoError(t, env.Client.Get(ctx, key, query))
			query.Spec.Replicas = 3
			require.NoError(t, env.Client.Update(ctx, query))
			require.Eventually(t, func() bool {
				deployment := &appsv1.Deployment{}
				err := env.Client.Get(ctx, client.ObjectKey{Name: controller.QueryNameFromParent(queryName), Namespace: namespace}, deployment)
				return err == nil && ptr.Deref(deployment.Spec.Replicas, 0) == 3
			}, time.Minute, 100*time.Millisecond)
		}
	})
}

func newQuery(namespace string) *v1alpha1.ThanosQuery {
	return &v1alpha1.ThanosQuery{
		ObjectMeta: metav1.ObjectMeta{Name: queryName, Namespace: namespace},
		Spec:       v1alpha1.ThanosQuerySpec{Replicas: 1},
	}
}

func startManager(t *testing.T, env *suite.Env, namespace string, gates featuregate.Config) {
	t.Helper()
	cacheOptions, err := controller.CacheOptionsForNamespace(namespace)
	require.NoError(t, err)
	mgr, err := ctrl.NewManager(env.Cfg, ctrl.Options{
		Scheme:     env.Scheme,
		Cache:      cacheOptions,
		Metrics:    metricsserver.Options{BindAddress: "0"},
		Controller: controllerconfig.Controller{SkipNameValidation: ptr.To(true)},
	})
	require.NoError(t, err)
	registry := prometheus.NewRegistry()
	conf := controller.Config{
		FeatureGate: gates,
		InstrumentationConfig: controller.InstrumentationConfig{
			Logger:          logr.Discard(),
			EventRecorder:   events.NewFakeRecorder(1000),
			MetricsRegistry: registry,
			CommonMetrics:   metrics.NewCommonMetrics(registry),
		},
	}
	query := controller.NewThanosQueryReconciler(conf, mgr.GetClient(), env.Scheme)
	require.NoError(t, query.SetupWithManager(mgr))
	status := controller.NewObjectStatusReconciler(conf, mgr.GetClient(), env.Scheme)
	require.NoError(t, status.SetupWithManager(mgr))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- mgr.Start(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(15 * time.Second):
			t.Error("manager did not stop")
		}
	})
	require.True(t, mgr.GetCache().WaitForCacheSync(ctx))
}
