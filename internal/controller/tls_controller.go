// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates;issuers,verbs=get;list;watch;create;update;patch;delete

package controller

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
)

const tlsManagedValue = "true"

// TLSReconciler maintains namespace trust and issued Secret ownership.
type TLSReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	featureGate featuregate.Config
}

func NewTLSReconciler(conf Config, c client.Client, scheme *runtime.Scheme) *TLSReconciler {
	return &TLSReconciler{
		Client:      c,
		Scheme:      scheme,
		featureGate: conf.FeatureGate,
	}
}

func (r *TLSReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if !r.featureGate.ServerTLSEnabled() || req.Namespace == "" {
		return ctrl.Result{}, nil
	}
	needed, err := r.hasThanosResources(ctx, req.Namespace)
	if err != nil || !needed {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, r.syncResources(ctx, req.Namespace)
}

func (r *TLSReconciler) syncResources(ctx context.Context, namespace string) error {
	cfg := r.featureGate.ServerTLS
	if err := cfg.Validate(); err != nil {
		return err
	}
	if cfg.Automatic() {
		if err := r.syncCAResources(ctx, namespace); err != nil {
			return err
		}
		if err := r.publishTrustBundle(ctx, namespace); err != nil {
			return err
		}
	}
	if _, err := readTLSTrustBundle(ctx, r.Client, namespace, cfg.CABundle()); err != nil {
		return err
	}
	return r.syncCertificateSecrets(ctx, namespace)
}

func (r *TLSReconciler) syncCAResources(ctx context.Context, namespace string) error {
	if err := checkTLSSecret(ctx, r.Client, featuregate.TLSCAName, namespace); err != nil {
		return err
	}
	// Shared trust must outlive individual workloads and Thanos resources.
	for _, obj := range manifests.BuildNamespaceTLSResources(namespace) {
		desired := obj.DeepCopyObject().(client.Object)
		mutate := manifests.MutateFuncFor(obj, desired)
		_, err := ctrl.CreateOrUpdate(ctx, r.Client, obj, func() error {
			if err := checkManagedTLSResource(obj, nil); err != nil {
				return err
			}
			return mutate()
		})
		if err != nil {
			return fmt.Errorf("reconciling TLS resource %s: %w", obj.GetName(), err)
		}
	}
	return nil
}

func (r *TLSReconciler) publishTrustBundle(ctx context.Context, namespace string) error {
	secret := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: featuregate.TLSCAName}, secret); err != nil {
		return fmt.Errorf("waiting for namespace CA: %w", err)
	}
	root := secret.Data[corev1.TLSCertKey]
	if err := validateTLSBundle(root); err != nil {
		return fmt.Errorf("namespace CA: %w", err)
	}
	bundle := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: featuregate.TLSCAName, Namespace: namespace}}
	_, err := ctrl.CreateOrUpdate(ctx, r.Client, bundle, func() error {
		if err := checkManagedTLSResource(bundle, nil); err != nil {
			return err
		}
		bundle.Labels = map[string]string{manifests.TLSLabel: tlsManagedValue}
		if bundle.Data == nil {
			bundle.Data = map[string]string{}
		}
		// Keep old roots while certificates issued before CA renewal remain in use.
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

func (r *TLSReconciler) hasThanosResources(ctx context.Context, namespace string) (bool, error) {
	for _, list := range []client.ObjectList{
		&v1alpha1.ThanosQueryList{}, &v1alpha1.ThanosReceiveList{}, &v1alpha1.ThanosStoreList{},
		&v1alpha1.ThanosRulerList{}, &v1alpha1.ThanosCompactList{},
	} {
		if err := r.List(ctx, list, client.InNamespace(namespace)); err != nil {
			return false, err
		}
		items, err := meta.ExtractList(list)
		if err != nil {
			return false, err
		}
		for _, item := range items {
			if item.(metav1.Object).GetDeletionTimestamp() == nil {
				return true, nil
			}
		}
	}
	return false, nil
}

func (r *TLSReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if !r.featureGate.ServerTLSEnabled() {
		return nil
	}
	enqueue := handler.EnqueueRequestsFromMapFunc(r.enqueueNamespace)
	b := ctrl.NewControllerManagedBy(mgr).Named("tls")
	for _, obj := range []client.Object{
		&v1alpha1.ThanosQuery{}, &v1alpha1.ThanosReceive{}, &v1alpha1.ThanosStore{},
		&v1alpha1.ThanosRuler{}, &v1alpha1.ThanosCompact{},
	} {
		b = b.Watches(obj, enqueue, builder.WithPredicates(predicate.GenerationChangedPredicate{}))
	}
	return b.Watches(&corev1.Secret{}, enqueue).
		Watches(&corev1.ConfigMap{}, enqueue).
		Watches(&cmv1.Certificate{}, enqueue).
		Watches(&cmv1.Issuer{}, enqueue).
		Complete(r)
}

func (r *TLSReconciler) enqueueNamespace(_ context.Context, obj client.Object) []reconcile.Request {
	if obj.GetNamespace() == "" {
		return nil
	}
	cfg := r.featureGate.ServerTLS
	switch obj.(type) {
	case *corev1.ConfigMap:
		if obj.GetName() != cfg.CABundle().Name {
			return nil
		}
	case *corev1.Secret, *cmv1.Certificate:
		if obj.GetName() == featuregate.TLSCAName {
			if !cfg.Automatic() {
				return nil
			}
		} else if obj.GetLabels()[manifests.TLSLabel] != tlsManagedValue {
			return nil
		}
	case *cmv1.Issuer:
		if !cfg.Automatic() || (obj.GetName() != featuregate.TLSCAName && obj.GetName() != manifests.TLSBootstrapIssuerName) {
			return nil
		}
	}
	return []reconcile.Request{{NamespacedName: client.ObjectKey{Namespace: obj.GetNamespace(), Name: featuregate.TLSCAName}}}
}

// TLS material is shared within a namespace; changes reconcile its Thanos resources.
// Watches are registered only when TLS is enabled, so cert-manager remains optional.
func withTLSWatches(b *builder.Builder, c client.Client, fg featuregate.Config, resourceList client.ObjectList) *builder.Builder {
	if !fg.ServerTLSEnabled() {
		return b
	}
	enqueue := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		_, configMap := obj.(*corev1.ConfigMap)
		if !configMap && obj.GetName() == featuregate.TLSCAName {
			return nil
		}
		if obj.GetLabels()[manifests.TLSLabel] != tlsManagedValue && !(configMap && obj.GetName() == fg.ServerTLS.CABundle().Name) {
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
		Watches(&cmv1.Certificate{}, enqueue)
}

// prepareTLSResources resolves trust and checks collisions before normal resource application.
func prepareTLSResources(ctx context.Context, c client.Client, fg featuregate.Config, owner client.Object, objects []client.Object) error {
	if !fg.ServerTLSEnabled() {
		return nil
	}
	bundle, err := readTLSTrustBundle(ctx, c, owner.GetNamespace(), fg.ServerTLS.CABundle())
	if err != nil {
		return err
	}
	checksum := fmt.Sprintf("%x", sha256.Sum256(bundle))
	for _, obj := range objects {
		if template := manifests.PodTemplate(obj); template != nil {
			if err := manifests.ValidateTLSWorkload(template); err != nil {
				return err
			}
			template.Annotations = manifests.MergeMaps(template.Annotations, map[string]string{manifests.TLSTrustChecksumAnnotation: checksum})
		}
		if obj.GetLabels()[manifests.TLSLabel] != tlsManagedValue {
			continue
		}
		switch obj := obj.(type) {
		case *cmv1.Certificate:
			if err := checkTLSSecret(ctx, c, obj.Spec.SecretName, owner.GetNamespace()); err != nil {
				return err
			}
		case *corev1.ConfigMap:
		default:
			continue
		}
		existing := obj.DeepCopyObject().(client.Object)
		key := client.ObjectKey{Namespace: owner.GetNamespace(), Name: obj.GetName()}
		if err := c.Get(ctx, key, existing); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		if err := checkManagedTLSResource(existing, owner); err != nil {
			return err
		}
	}
	return nil
}

// syncCertificateSecrets keeps leaf Secrets tied to their Certificates for garbage collection.
func (r *TLSReconciler) syncCertificateSecrets(ctx context.Context, namespace string) error {
	list := &cmv1.CertificateList{}
	if err := r.List(ctx, list, client.InNamespace(namespace), client.MatchingLabels{manifests.TLSLabel: tlsManagedValue}); err != nil {
		return err
	}
	for _, cert := range list.Items {
		owner := metav1.GetControllerOf(&cert)
		if owner == nil || owner.APIVersion != v1alpha1.GroupVersion.String() {
			continue
		}
		switch owner.Kind {
		case "ThanosQuery", "ThanosReceive", "ThanosStore", "ThanosRuler", "ThanosCompact":
		default:
			continue
		}
		if err := checkTLSSecret(ctx, r.Client, cert.Spec.SecretName, namespace); err != nil {
			return err
		}
		secret := &corev1.Secret{}
		if err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: cert.Spec.SecretName}, secret); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		before := secret.DeepCopy()
		if err := controllerutil.SetControllerReference(&cert, secret, r.Scheme); err != nil {
			return err
		}
		if !metav1.IsControlledBy(before, &cert) {
			if err := r.Patch(ctx, secret, client.MergeFrom(before)); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkTLSSecret refuses to reuse a Secret outside the TLS integration.
func checkTLSSecret(ctx context.Context, c client.Client, name, namespace string) error {
	secret := &corev1.Secret{}
	if err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, secret); err != nil {
		return client.IgnoreNotFound(err)
	}
	if secret.Labels[manifests.TLSLabel] != tlsManagedValue || secret.Annotations["cert-manager.io/certificate-name"] != name {
		return fmt.Errorf("TLS Secret %s/%s already exists outside this integration", namespace, name)
	}
	return nil
}

// readTLSTrustBundle reads and validates the configured public CA bundle.
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

// checkManagedTLSResource checks existing resources before applying TLS changes.
func checkManagedTLSResource(obj, owner client.Object) error {
	if obj.GetResourceVersion() == "" {
		return nil
	}
	if obj.GetLabels()[manifests.TLSLabel] != tlsManagedValue || (owner != nil && !metav1.IsControlledBy(obj, owner)) {
		return fmt.Errorf("TLS resource %s/%s already exists outside this integration", obj.GetNamespace(), obj.GetName())
	}
	return nil
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
