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

func TestThanosQueryServiceMonitorCleanup(t *testing.T) {
	const owner, namespace = "stack", "monitoring"
	queryName, frontendName := QueryNameFromParent(owner), QueryFrontendNameFromParent(owner)
	query := v1alpha1.ThanosQuery{
		ObjectMeta: metav1.ObjectMeta{Name: owner, Namespace: namespace, UID: "query-uid"},
		Spec:       v1alpha1.ThanosQuerySpec{Replicas: 2, QueryFrontend: &v1alpha1.QueryFrontendSpec{Replicas: 2}},
	}
	queryRef := *metav1.NewControllerRef(&query, v1alpha1.GroupVersion.WithKind("ThanosQuery"))
	previousRef := queryRef
	previousRef.UID = "previous-query-uid"
	receiveRef := *metav1.NewControllerRef(&v1alpha1.ThanosReceive{
		ObjectMeta: metav1.ObjectMeta{Name: owner, Namespace: namespace, UID: "receive-uid"},
	}, v1alpha1.GroupVersion.WithKind("ThanosReceive"))

	scheme := runtime.NewScheme()
	assert.NilError(t, clientgoscheme.AddToScheme(scheme))
	assert.NilError(t, monitoringv1.AddToScheme(scheme))

	for _, tc := range []struct {
		name            string
		serviceMonitors bool
		wantMonitors    []string
	}{
		{
			name:            "enabled",
			serviceMonitors: true,
			wantMonitors:    []string{queryName, frontendName, "old-monitor", "receive", "previous-owner", "unowned"},
		},
		{
			name:         "disabled",
			wantMonitors: []string{"receive", "previous-owner", "unowned"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var objects []client.Object
			for _, name := range []string{queryName, frontendName, "old-monitor"} {
				objects = append(objects, &monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{
					Name: name, Namespace: namespace, OwnerReferences: []metav1.OwnerReference{queryRef},
				}})
			}
			objects = append(objects,
				&monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{
					Name: "receive", Namespace: namespace, OwnerReferences: []metav1.OwnerReference{receiveRef},
				}},
				&monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{
					Name: "previous-owner", Namespace: namespace, OwnerReferences: []metav1.OwnerReference{previousRef},
				}},
				&monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{
					Name: "unowned", Namespace: namespace,
				}},
				&monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{
					Name: frontendName, Namespace: "other", OwnerReferences: []metav1.OwnerReference{queryRef},
				}},
				&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: queryName, Namespace: namespace}},
			)
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
			gates := featuregate.Config{
				ServiceMonitor: &featuregate.ServiceMonitorConfig{FeatureConfig: featuregate.FeatureConfig{Enabled: tc.serviceMonitors}},
			}
			r := ThanosQueryReconciler{
				Client:      c,
				handler:     handlers.NewHandler(c, scheme, logr.Discard()).SetFeatureGates(gates.ToGVK()),
				featureGate: gates,
			}

			ctx := context.Background()
			assert.Equal(t, r.cleanup(ctx, query, []string{queryName, frontendName}), 0)

			var monitors monitoringv1.ServiceMonitorList
			assert.NilError(t, c.List(ctx, &monitors, client.InNamespace(namespace)))
			names := make([]string, 0, len(monitors.Items))
			for _, monitor := range monitors.Items {
				names = append(names, monitor.Name)
			}
			slices.Sort(names)
			slices.Sort(tc.wantMonitors)
			assert.DeepEqual(t, names, tc.wantMonitors)
			assert.NilError(t, c.Get(ctx, client.ObjectKey{Name: frontendName, Namespace: "other"}, &monitoringv1.ServiceMonitor{}))
			assert.NilError(t, c.Get(ctx, client.ObjectKey{Name: queryName, Namespace: namespace}, &corev1.Service{}))
		})
	}
}
