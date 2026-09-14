// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates;issuers,verbs=get;list;watch;create;update;patch;delete

package controller

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
)

const tlsManagedValue = "true"

// TLSReconciler maintains namespace trust.
type TLSReconciler struct {
	client.Client

	featureGate featuregate.Config
	namespace   string
}

// NewTLSReconciler creates a trust reconciler scoped to the configured watch namespace.
func NewTLSReconciler(conf Config, c client.Client) *TLSReconciler {
	return &TLSReconciler{
		Client:      c,
		featureGate: conf.FeatureGate,
		namespace:   conf.WatchNamespace,
	}
}

// Reconcile handles startup and resource events by syncing trust in the configured namespace.
func (r *TLSReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if !r.featureGate.ServerTLSEnabled() || r.namespace == "" || req.Namespace != r.namespace {
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, r.syncResources(ctx)
}

// syncResources validates TLS settings and trust, provisioning and publishing the CA in automatic mode.
func (r *TLSReconciler) syncResources(ctx context.Context) error {
	cfg := r.featureGate.ServerTLS
	if err := cfg.Validate(); err != nil {
		return err
	}
	if cfg.Automatic() {
		if err := r.syncCAResources(ctx); err != nil {
			return err
		}
		if err := r.publishTrustBundle(ctx); err != nil {
			return err
		}
	}
	_, err := readTLSTrustBundle(ctx, r.Client, r.namespace, cfg.CABundle())
	return err
}

// syncCAResources creates or updates the shared CA Certificate and bootstrap and signing Issuers.
// These resources have no component owner, so they survive component deletion.
func (r *TLSReconciler) syncCAResources(ctx context.Context) error {
	for _, obj := range manifests.BuildNamespaceTLSResources(r.namespace) {
		desired := obj.DeepCopyObject().(client.Object)
		_, err := ctrl.CreateOrUpdate(ctx, r.Client, obj, manifests.MutateFuncFor(obj, desired))
		if err != nil {
			return fmt.Errorf("reconciling TLS resource %s: %w", obj.GetName(), err)
		}
	}
	return nil
}

// publishTrustBundle copies public CA certificates from the issued Secret into the trust ConfigMap.
// Previous roots are retained so existing leaf certificates remain trusted after CA renewal.
func (r *TLSReconciler) publishTrustBundle(ctx context.Context) error {
	secret := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: r.namespace, Name: featuregate.TLSCAName}, secret); err != nil {
		return fmt.Errorf("waiting for namespace CA: %w", err)
	}
	root := secret.Data[corev1.TLSCertKey]
	if err := validateTLSBundle(root); err != nil {
		return fmt.Errorf("namespace CA: %w", err)
	}
	bundle := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: featuregate.TLSCAName, Namespace: r.namespace}}
	_, err := ctrl.CreateOrUpdate(ctx, r.Client, bundle, func() error {
		bundle.Labels = map[string]string{manifests.TLSLabel: tlsManagedValue}
		if bundle.Data == nil {
			bundle.Data = map[string]string{}
		}
		old := bundle.Data[featuregate.TLSCAKey]
		if !bytes.Contains([]byte(old), bytes.TrimSpace(root)) {
			bundle.Data[featuregate.TLSCAKey] = old + "\n" + string(root)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("publishing namespace CA: %w", err)
	}
	return nil
}

// SetupWithManager registers shared trust watches when TLS is enabled and queues a startup request.
// The startup request allows trust to be prepared before any component resources exist.
func (r *TLSReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if !r.featureGate.ServerTLSEnabled() {
		return nil
	}
	if _, err := CacheOptionsForNamespace(r.namespace, r.featureGate); err != nil {
		return err
	}
	enqueue := handler.EnqueueRequestsFromMapFunc(r.enqueueNamespace)
	b := ctrl.NewControllerManagedBy(mgr).Named("tls").
		WatchesRawSource(source.Func(func(_ context.Context, queue workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
			// Bootstrap before any component resources exist.
			queue.Add(ctrl.Request{NamespacedName: client.ObjectKey{Namespace: r.namespace, Name: featuregate.TLSCAName}})
			return nil
		}))
	if r.featureGate.ServerTLS.Automatic() {
		b = b.Watches(&corev1.Secret{}, enqueue).
			Watches(&cmv1.Certificate{}, enqueue).
			Watches(&cmv1.Issuer{}, enqueue)
	}
	return b.Watches(&corev1.ConfigMap{}, enqueue).Complete(r)
}

// enqueueNamespace maps relevant shared trust events to a single request for the configured namespace.
func (r *TLSReconciler) enqueueNamespace(_ context.Context, obj client.Object) []reconcile.Request {
	if obj.GetNamespace() != r.namespace {
		return nil
	}
	cfg := r.featureGate.ServerTLS
	switch obj.(type) {
	case *corev1.ConfigMap:
		if obj.GetName() != cfg.CABundle().Name {
			return nil
		}
	case *corev1.Secret, *cmv1.Certificate:
		if !cfg.Automatic() || obj.GetName() != featuregate.TLSCAName {
			return nil
		}
	case *cmv1.Issuer:
		if !cfg.Automatic() || (obj.GetName() != featuregate.TLSCAName && obj.GetName() != manifests.TLSBootstrapIssuerName) {
			return nil
		}
	default:
		return nil
	}
	return []reconcile.Request{{NamespacedName: client.ObjectKey{Namespace: obj.GetNamespace(), Name: featuregate.TLSCAName}}}
}

// withTLSWatches adds component watches for owned Certificates and the trust ConfigMap when TLS is enabled.
// Trust changes requeue components in the same namespace to refresh their pod-template checksums.
func withTLSWatches(b *builder.Builder, c client.Client, fg featuregate.Config, resourceList client.ObjectList) *builder.Builder {
	if !fg.ServerTLSEnabled() {
		return b
	}
	enqueue := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		if obj.GetName() != fg.ServerTLS.CABundle().Name {
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
	return b.Owns(&cmv1.Certificate{}).Watches(&corev1.ConfigMap{}, enqueue)
}

// readTLSTrustBundle reads the configured ConfigMap key and validates its public CA bundle.
func readTLSTrustBundle(ctx context.Context, c client.Client, namespace string, ref featuregate.CABundleReference) ([]byte, error) {
	bundle := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: ref.Name}, bundle); err != nil {
		return nil, fmt.Errorf("reading TLS trust ConfigMap: %w", err)
	}
	data := []byte(bundle.Data[ref.Key])
	if err := validateTLSBundle(data); err != nil {
		return nil, err
	}
	return data, nil
}

// validateTLSBundle requires a nonempty PEM bundle of CA certificates.
func validateTLSBundle(bundle []byte) error {
	count := 0
	for len(bytes.TrimSpace(bundle)) > 0 {
		block, rest := pem.Decode(bundle)
		if block == nil || block.Type != "CERTIFICATE" {
			return fmt.Errorf("TLS trust bundle must contain PEM CA certificates")
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil || !cert.IsCA {
			return fmt.Errorf("TLS trust bundle contains an invalid CA certificate")
		}
		count++
		bundle = rest
	}
	if count == 0 {
		return fmt.Errorf("TLS trust bundle is empty")
	}
	return nil
}
