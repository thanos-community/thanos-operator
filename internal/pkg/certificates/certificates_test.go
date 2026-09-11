package certificates

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
)

func newManager(t *testing.T) Manager {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{cmv1.AddToScheme, corev1.AddToScheme, appsv1.AddToScheme} {
		require.NoError(t, add(scheme))
	}
	return Manager{Client: fake.NewClientBuilder().WithScheme(scheme).Build(), Scheme: scheme, Config: featuregate.ServerTLSConfig{Provider: featuregate.CertManagerProvider}}
}

func rootPEM(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)
	cert := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "test CA"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func managedSecret(name string, data []byte) *corev1.Secret {
	return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test", Labels: labels(), Annotations: map[string]string{"cert-manager.io/certificate-name": name}}, Data: map[string][]byte{corev1.TLSCertKey: data}}
}

func TestAutomaticCAAndRenewal(t *testing.T) {
	ctx := context.Background()
	m := newManager(t)
	require.ErrorContains(t, m.EnsureNamespace(ctx, "test"), "waiting for namespace CA")
	root := rootPEM(t)
	secret := managedSecret(featuregate.TLSCAName, root)
	require.NoError(t, m.Client.Create(ctx, secret))
	require.NoError(t, m.EnsureNamespace(ctx, "test"))
	bundle := &corev1.ConfigMap{}
	key := client.ObjectKey{Namespace: "test", Name: featuregate.TLSCAName}
	require.NoError(t, m.Client.Get(ctx, key, bundle))
	require.Empty(t, bundle.OwnerReferences)
	require.Contains(t, bundle.Data[featuregate.TLSCAKey], string(root))
	version := bundle.ResourceVersion
	require.NoError(t, m.EnsureNamespace(ctx, "test"))
	require.NoError(t, m.Client.Get(ctx, key, bundle))
	require.Equal(t, version, bundle.ResourceVersion, "unchanged trust must not trigger updates")
	newRoot := rootPEM(t)
	secret.Data[corev1.TLSCertKey] = newRoot
	require.NoError(t, m.Client.Update(ctx, secret))
	require.NoError(t, m.EnsureNamespace(ctx, "test"))
	require.NoError(t, m.Client.Get(ctx, key, bundle))
	require.Contains(t, bundle.Data[featuregate.TLSCAKey], string(root))
	require.Contains(t, bundle.Data[featuregate.TLSCAKey], string(newRoot))
}

func TestExternalIssuerAndWorkloadLifecycle(t *testing.T) {
	ctx := context.Background()
	m := newManager(t)
	m.Config.CertManager = &featuregate.CertManagerConfig{IssuerRef: &featuregate.IssuerReference{Name: "platform", Kind: "ClusterIssuer"}, CABundleConfigMap: &featuregate.CABundleReference{Name: "trust", Key: "roots.pem"}}
	bundle := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "trust", Namespace: "test"}, Data: map[string]string{"roots.pem": string(rootPEM(t))}}
	require.NoError(t, m.Client.Create(ctx, bundle))
	require.NoError(t, m.EnsureNamespace(ctx, "test"))
	issuers := &cmv1.IssuerList{}
	require.NoError(t, m.Client.List(ctx, issuers))
	require.Empty(t, issuers.Items)
	workload := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "receive", Namespace: "test", UID: "workload-uid"}, Spec: appsv1.StatefulSetSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Args: []string{"receive"}}}}}}}
	require.NoError(t, m.EnsureWorkload(ctx, workload))
	cert := &cmv1.Certificate{}
	key := client.ObjectKey{Name: manifests.TLSResourceName(workload.Name), Namespace: "test"}
	require.NoError(t, m.Client.Get(ctx, key, cert))
	require.True(t, metav1.IsControlledBy(cert, workload))
	require.Equal(t, "ClusterIssuer", cert.Spec.IssuerRef.Kind)
	require.Equal(t, []string{"receive.test.svc", "*.receive.test.svc"}, cert.Spec.DNSNames)
	require.Equal(t, []cmv1.KeyUsage{cmv1.UsageDigitalSignature, cmv1.UsageServerAuth}, cert.Spec.Usages)
	require.NoError(t, m.SetTrustChecksum(ctx, workload))
	initialTemplate := workload.Spec.Template.DeepCopy()
	first := initialTemplate.Annotations[manifests.TLSTrustChecksumAnnotation]
	require.NotEmpty(t, first, "trust checksum must exist before leaf issuance")
	secret := managedSecret(key.Name, []byte("certificate one"))
	require.NoError(t, m.Client.Create(ctx, secret))
	require.NoError(t, m.EnsureWorkload(ctx, workload))
	require.NoError(t, m.SetTrustChecksum(ctx, workload))
	require.Equal(t, *initialTemplate, workload.Spec.Template, "initial issuance must not roll pods")
	require.NoError(t, m.SetTrustChecksum(ctx, workload))
	require.Equal(t, *initialTemplate, workload.Spec.Template, "unchanged trust must keep the template stable")
	require.NoError(t, m.Client.Get(ctx, key, secret))
	require.Equal(t, workload.UID, secret.OwnerReferences[0].UID)
	secret.Data[corev1.TLSCertKey] = []byte("certificate two")
	secret.Data[corev1.TLSPrivateKeyKey] = []byte("rotated private key")
	require.NoError(t, m.Client.Update(ctx, secret))
	require.NoError(t, m.SetTrustChecksum(ctx, workload))
	require.Equal(t, *initialTemplate, workload.Spec.Template, "leaf renewal must not roll pods")
	bundle.Data["roots.pem"] += string(rootPEM(t))
	require.NoError(t, m.Client.Update(ctx, bundle))
	require.NoError(t, m.SetTrustChecksum(ctx, workload))
	require.NotEqual(t, first, workload.Spec.Template.Annotations[manifests.TLSTrustChecksumAnnotation], "trust changes must refresh clients")
	require.NoError(t, m.CleanupWorkload(ctx, workload))
	require.True(t, apierrors.IsNotFound(m.Client.Get(ctx, key, &corev1.Secret{})))
	require.True(t, apierrors.IsNotFound(m.Client.Get(ctx, key, &cmv1.Certificate{})))
	require.True(t, apierrors.IsNotFound(m.Client.Get(ctx, key, &corev1.ConfigMap{})))
	require.NoError(t, m.Client.Get(ctx, client.ObjectKeyFromObject(bundle), &corev1.ConfigMap{}), "external trust must survive disable")
}

func TestRefusesUnmanagedResources(t *testing.T) {
	ctx := context.Background()
	m := newManager(t)
	issuer := &cmv1.Issuer{ObjectMeta: metav1.ObjectMeta{Name: bootstrapIssuer, Namespace: "test"}}
	require.NoError(t, m.Client.Create(ctx, issuer))
	require.ErrorContains(t, m.EnsureNamespace(ctx, "test"), "outside this integration")
	m = newManager(t)
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: featuregate.TLSCAName, Namespace: "test"}}
	require.NoError(t, m.Client.Create(ctx, secret))
	require.ErrorContains(t, m.EnsureNamespace(ctx, "test"), "outside this integration")
}
