// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates;issuers,verbs=get;list;watch;create;update;patch;delete

package controller

import (
	"context"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
)

// TLS material is shared within a namespace; changes reconcile its Thanos resources.
// Watches are registered only when TLS is enabled, so cert-manager remains optional.
func withTLSWatches(b *builder.Builder, c client.Client, fg featuregate.Config, resourceList client.ObjectList) *builder.Builder {
	if !fg.TLSEnabled() {
		return b
	}
	enqueue := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		if obj.GetLabels()[manifests.TLSLabel] != "true" && obj.GetName() != fg.TLS.CABundle().Name {
			return nil
		}
		list := resourceList.DeepCopyObject().(client.ObjectList)
		if err := c.List(ctx, list, client.InNamespace(obj.GetNamespace())); err != nil {
			log.FromContext(ctx).Error(err, "listing Thanos resources after TLS change")
			return nil
		}
		items, err := meta.ExtractList(list)
		if err != nil {
			return nil
		}
		requests := make([]reconcile.Request, 0, len(items))
		for _, item := range items {
			requests = append(requests, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(item.(client.Object))})
		}
		return requests
	})
	return b.Watches(&corev1.Secret{}, enqueue).Watches(&corev1.ConfigMap{}, enqueue).
		Watches(&cmv1.Certificate{}, enqueue).Watches(&cmv1.Issuer{}, enqueue)
}
