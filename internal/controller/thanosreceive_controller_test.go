package controller

import (
	"context"
	"slices"
	"testing"

	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/internal/pkg/handlers"
)

func TestThanosReceiveServiceMonitorCleanup(t *testing.T) {
	const owner, namespace = "stack", "monitoring"
	ingesterName, routerName := ReceiveIngesterNameFromParent(owner, "default"), ReceiveRouterNameFromParent(owner)
	syncName := routerName + "-kube-resource-sync"
	receive := v1alpha1.ThanosReceive{
		ObjectMeta: metav1.ObjectMeta{Name: owner, Namespace: namespace, UID: "receive-uid"},
		Spec:       v1alpha1.ThanosReceiveSpec{Router: v1alpha1.RouterSpec{Replicas: 2}},
	}
	receiveRef := *metav1.NewControllerRef(&receive, v1alpha1.GroupVersion.WithKind("ThanosReceive"))
	queryRef := *metav1.NewControllerRef(&v1alpha1.ThanosQuery{
		ObjectMeta: metav1.ObjectMeta{Name: owner, Namespace: namespace, UID: "query-uid"},
	}, v1alpha1.GroupVersion.WithKind("ThanosQuery"))

	scheme := runtime.NewScheme()
	assert.NilError(t, clientgoscheme.AddToScheme(scheme))
	assert.NilError(t, monitoringv1.AddToScheme(scheme))

	for _, tc := range []struct {
		name            string
		serviceMonitors bool
		resourceSync    bool
		wantMonitors    []string
	}{
		{
			name:            "both enabled",
			serviceMonitors: true,
			resourceSync:    true,
			wantMonitors:    []string{ingesterName, routerName, syncName, "query", "unowned"},
		},
		{
			name:         "monitors disabled",
			resourceSync: true,
			wantMonitors: []string{"query", "unowned"},
		},
		{
			name:            "resource sync disabled",
			serviceMonitors: true,
			wantMonitors:    []string{ingesterName, routerName, "query", "unowned"},
		},
		{
			name:         "both disabled",
			wantMonitors: []string{"query", "unowned"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var objects []client.Object
			for _, name := range []string{ingesterName, routerName, syncName} {
				objects = append(objects, &monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{
					Name: name, Namespace: namespace, OwnerReferences: []metav1.OwnerReference{receiveRef},
				}})
			}
			objects = append(objects,
				&monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{
					Name: "query", Namespace: namespace, OwnerReferences: []metav1.OwnerReference{queryRef},
				}},
				&monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{
					Name: "unowned", Namespace: namespace,
				}},
				&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: routerName, Namespace: namespace}},
			)
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
			gates := featuregate.Config{
				ServiceMonitor:   &featuregate.ServiceMonitorConfig{FeatureConfig: featuregate.FeatureConfig{Enabled: tc.serviceMonitors}},
				KubeResourceSync: &featuregate.KubeResourceSyncConfig{FeatureConfig: featuregate.FeatureConfig{Enabled: tc.resourceSync}},
			}
			r := ThanosReceiveReconciler{
				Client:      c,
				handler:     handlers.NewHandler(c, scheme, logr.Discard()).SetFeatureGates(gates.ToGVK()),
				featureGate: gates,
			}

			ctx := context.Background()
			assert.Equal(t, r.cleanup(ctx, receive, []string{ingesterName}, routerName), 0)

			var monitors monitoringv1.ServiceMonitorList
			assert.NilError(t, c.List(ctx, &monitors, client.InNamespace(namespace)))
			names := make([]string, 0, len(monitors.Items))
			for _, monitor := range monitors.Items {
				names = append(names, monitor.Name)
			}
			slices.Sort(names)
			slices.Sort(tc.wantMonitors)
			assert.DeepEqual(t, names, tc.wantMonitors)
			assert.NilError(t, c.Get(ctx, client.ObjectKey{Name: routerName, Namespace: namespace}, &corev1.Service{}))
		})
	}
}
