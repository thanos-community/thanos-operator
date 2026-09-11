// Package certificates reconciles cert-manager resources for Thanos TLS.
package certificates

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
)

const (
	bootstrapIssuer = "thanos-operator-selfsigned"
	managedValue    = "true"
)

type Manager struct {
	Client client.Client
	Scheme *runtime.Scheme
	Config featuregate.ServerTLSConfig
}

func labels() map[string]string { return map[string]string{manifests.TLSLabel: managedValue} }

func objectMeta(name, namespace string) metav1.ObjectMeta {
	return metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels()}
}

// EnsureNamespace leaves shared CA resources independent of any single workload.
func (m Manager) EnsureNamespace(ctx context.Context, namespace string) error {
	if err := m.Config.Validate(); err != nil {
		return err
	}
	if m.Config.Automatic() {
		if err := m.checkSecret(ctx, featuregate.TLSCAName, namespace); err != nil {
			return err
		}
		issuer := &cmv1.Issuer{ObjectMeta: objectMeta(bootstrapIssuer, namespace), Spec: cmv1.IssuerSpec{IssuerConfig: cmv1.IssuerConfig{SelfSigned: &cmv1.SelfSignedIssuer{}}}}
		if err := m.apply(ctx, issuer, nil); err != nil {
			return err
		}
		ca := &cmv1.Certificate{ObjectMeta: objectMeta(featuregate.TLSCAName, namespace), Spec: cmv1.CertificateSpec{
			SecretName: featuregate.TLSCAName, IsCA: true,
			CommonName:     manifests.SanitizeName("thanos-ca-" + namespace),
			Duration:       &metav1.Duration{Duration: 10 * 365 * 24 * time.Hour},
			RenewBefore:    &metav1.Duration{Duration: 365 * 24 * time.Hour},
			IssuerRef:      cmmeta.ObjectReference{Name: bootstrapIssuer, Kind: "Issuer", Group: "cert-manager.io"},
			PrivateKey:     &cmv1.CertificatePrivateKey{Algorithm: cmv1.ECDSAKeyAlgorithm, Size: 256, RotationPolicy: cmv1.RotationPolicyNever},
			SecretTemplate: &cmv1.CertificateSecretTemplate{Labels: labels()},
		}}
		if err := m.apply(ctx, ca, nil); err != nil {
			return err
		}
		issuer = &cmv1.Issuer{ObjectMeta: objectMeta(featuregate.TLSCAName, namespace), Spec: cmv1.IssuerSpec{IssuerConfig: cmv1.IssuerConfig{CA: &cmv1.CAIssuer{SecretName: featuregate.TLSCAName}}}}
		if err := m.apply(ctx, issuer, nil); err != nil {
			return err
		}
		secret := &corev1.Secret{}
		if err := m.Client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: featuregate.TLSCAName}, secret); err != nil {
			return fmt.Errorf("waiting for namespace CA: %w", err)
		}
		root := secret.Data[corev1.TLSCertKey]
		if err := validateBundle(root); err != nil {
			return fmt.Errorf("namespace CA: %w", err)
		}
		bundle := &corev1.ConfigMap{ObjectMeta: objectMeta(featuregate.TLSCAName, namespace)}
		_, err := controllerutil.CreateOrUpdate(ctx, m.Client, bundle, func() error {
			if err := checkManaged(bundle, nil); err != nil {
				return err
			}
			bundle.Labels = labels()
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
	}
	ref := m.Config.CABundle()
	bundle := &corev1.ConfigMap{}
	if err := m.Client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: ref.Name}, bundle); err != nil {
		return fmt.Errorf("reading TLS trust ConfigMap: %w", err)
	}
	return validateBundle([]byte(bundle.Data[ref.Key]))
}

// EnsureWorkload creates resources owned by the workload, including for each shard.
func (m Manager) EnsureWorkload(ctx context.Context, workload client.Object) error {
	name := manifests.TLSResourceName(workload.GetName())
	namespace := workload.GetNamespace()
	if err := m.checkSecret(ctx, name, namespace); err != nil {
		return err
	}
	dnsName := manifests.ServiceDNSName(workload.GetName(), namespace)
	dnsNames := []string{dnsName}
	// Receive forwards to individual StatefulSet pods through their headless service.
	if t := manifests.PodTemplate(workload); t != nil && t.Spec.Containers[0].Args[0] == "receive" {
		dnsNames = append(dnsNames, "*."+dnsName)
	}
	issuer := m.Config.Issuer()
	cert := &cmv1.Certificate{ObjectMeta: objectMeta(name, namespace), Spec: cmv1.CertificateSpec{
		SecretName: name, DNSNames: dnsNames,
		IssuerRef:      cmmeta.ObjectReference{Name: issuer.Name, Kind: issuer.Kind, Group: issuer.Group},
		Usages:         []cmv1.KeyUsage{cmv1.UsageDigitalSignature, cmv1.UsageServerAuth},
		PrivateKey:     &cmv1.CertificatePrivateKey{Algorithm: cmv1.ECDSAKeyAlgorithm, Size: 256, RotationPolicy: cmv1.RotationPolicyAlways},
		SecretTemplate: &cmv1.CertificateSecretTemplate{Labels: labels()},
	}}
	if err := m.apply(ctx, cert, workload); err != nil {
		return err
	}
	// cert-manager's Secret owner references are optional. Also link to the
	// workload so deleting a shard always removes its private key.
	secret := &corev1.Secret{}
	if err := m.Client.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	} else {
		before := secret.DeepCopy()
		if err := controllerutil.SetOwnerReference(workload, secret, m.Scheme); err != nil {
			return err
		}
		if err := m.Client.Patch(ctx, secret, client.MergeFrom(before)); err != nil {
			return err
		}
	}
	web := &corev1.ConfigMap{ObjectMeta: objectMeta(name, namespace), Data: map[string]string{"http.yaml": manifests.TLSWebConfig}}
	return m.apply(ctx, web, workload)
}

func (m Manager) checkSecret(ctx context.Context, name, namespace string) error {
	secret := &corev1.Secret{}
	if err := m.Client.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, secret); err != nil {
		return client.IgnoreNotFound(err)
	}
	if secret.Labels[manifests.TLSLabel] != managedValue || secret.Annotations["cert-manager.io/certificate-name"] != name {
		return fmt.Errorf("TLS Secret %s/%s already exists outside this integration", namespace, name)
	}
	return nil
}

// CleanupWorkload retains shared trust while removing unused leaf resources.
func (m Manager) CleanupWorkload(ctx context.Context, workload client.Object) error {
	key := client.ObjectKey{Namespace: workload.GetNamespace(), Name: manifests.TLSResourceName(workload.GetName())}
	web := &corev1.ConfigMap{}
	if err := m.Client.Get(ctx, key, web); err != nil {
		return client.IgnoreNotFound(err)
	}
	if web.Labels[manifests.TLSLabel] != managedValue || !metav1.IsControlledBy(web, workload) {
		return nil
	}
	cert := &cmv1.Certificate{}
	if err := m.Client.Get(ctx, key, cert); err != nil {
		if !apierrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
			return err
		}
	} else {
		if err := checkManaged(cert, workload); err != nil {
			return err
		}
		if err := client.IgnoreNotFound(m.Client.Delete(ctx, cert)); err != nil {
			return err
		}
	}
	secret := &corev1.Secret{}
	if err := m.Client.Get(ctx, key, secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	} else if secret.Labels[manifests.TLSLabel] == managedValue && secret.Annotations["cert-manager.io/certificate-name"] == key.Name {
		if err := client.IgnoreNotFound(m.Client.Delete(ctx, secret)); err != nil {
			return err
		}
	}
	return client.IgnoreNotFound(m.Client.Delete(ctx, web))
}

// SetTrustChecksum refreshes clients that load their CA bundle only at startup.
// Thanos reloads leaf certificates from the mounted Secret without a rollout.
func (m Manager) SetTrustChecksum(ctx context.Context, workload client.Object) error {
	ref := m.Config.CABundle()
	bundle := &corev1.ConfigMap{}
	if err := m.Client.Get(ctx, client.ObjectKey{Namespace: workload.GetNamespace(), Name: ref.Name}, bundle); err != nil {
		return err
	}
	hash := sha256.Sum256([]byte(bundle.Data[ref.Key]))
	template := manifests.PodTemplate(workload)
	template.Annotations = manifests.MergeMaps(template.Annotations, map[string]string{manifests.TLSTrustChecksumAnnotation: fmt.Sprintf("%x", hash)})
	return nil
}

func (m Manager) apply(ctx context.Context, obj, owner client.Object) error {
	desired := obj.DeepCopyObject().(client.Object)
	_, err := controllerutil.CreateOrUpdate(ctx, m.Client, obj, func() error {
		if err := checkManaged(obj, owner); err != nil {
			return err
		}
		obj.SetLabels(labels())
		if owner != nil {
			if err := controllerutil.SetControllerReference(owner, obj, m.Scheme); err != nil {
				return err
			}
		}
		switch obj := obj.(type) {
		case *cmv1.Certificate:
			obj.Spec = desired.(*cmv1.Certificate).Spec
		case *cmv1.Issuer:
			obj.Spec = desired.(*cmv1.Issuer).Spec
		case *corev1.ConfigMap:
			obj.Data = desired.(*corev1.ConfigMap).Data
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("reconciling TLS resource %s: %w", obj.GetName(), err)
	}
	return nil
}

func checkManaged(obj, owner client.Object) error {
	if obj.GetResourceVersion() == "" {
		return nil
	}
	if obj.GetLabels()[manifests.TLSLabel] != managedValue || (owner != nil && !metav1.IsControlledBy(obj, owner)) {
		return fmt.Errorf("TLS resource %s/%s already exists outside this integration", obj.GetNamespace(), obj.GetName())
	}
	return nil
}

func validateBundle(bundle []byte) error {
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
