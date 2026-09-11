package controller

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
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/internal/pkg/handlers"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/query"
	manifestsstore "github.com/thanos-community/thanos-operator/internal/pkg/manifests/store"
)

func TestTLSFanoutUsesReadyEndpointSliceTargets(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, discoveryv1.AddToScheme(scheme))
	slice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{Name: "store-a", Namespace: "metrics", Labels: map[string]string{discoveryv1.LabelServiceName: "store"}},
		Ports:      []discoveryv1.EndpointPort{{Name: ptr.To("grpc"), Port: ptr.To(int32(11901))}},
		Endpoints: []discoveryv1.Endpoint{
			{Addresses: []string{"10.0.0.1"}, Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true)}},
			{Addresses: []string{"10.0.0.2"}, Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(false)}},
			{Addresses: []string{"2001:db8::1"}},
		},
	}
	duplicate := slice.DeepCopy()
	duplicate.Name = "store-b"
	otherNamespace := slice.DeepCopy()
	otherNamespace.Namespace = "other"
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(slice, duplicate, otherNamespace).Build()
	r := &ThanosQueryReconciler{Client: c, handler: handlers.NewHandler(c, scheme, logr.Discard())}
	endpoints, err := r.resolveTLSFanout(context.Background(), query.Endpoint{ServiceName: "store", Namespace: "metrics", Port: 10901, Type: manifests.RegularLabel})
	require.NoError(t, err)
	require.Len(t, endpoints, 2)
	require.Equal(t, "10.0.0.1:11901", endpoints[0].Address)
	require.Equal(t, "[2001:db8::1]:11901", endpoints[1].Address)
	for _, ep := range endpoints {
		require.Equal(t, "store", ep.ServiceName)
		require.Equal(t, "metrics", ep.Namespace)
		require.Equal(t, manifests.RegularLabel, ep.Type)
	}
	require.NoError(t, c.Delete(context.Background(), slice))
	require.NoError(t, c.Delete(context.Background(), duplicate))
	endpoints, err = r.resolveTLSFanout(context.Background(), query.Endpoint{ServiceName: "store", Namespace: "metrics", Type: manifests.RegularLabel})
	require.NoError(t, err)
	require.Empty(t, endpoints)
}

func TestTLSEventSelection(t *testing.T) {
	for _, external := range []bool{false, true} {
		cfg := featuregate.ServerTLSConfig{FeatureConfig: featuregate.FeatureConfig{Enabled: true}}
		if external {
			cfg.CertManager = &featuregate.CertManagerConfig{
				IssuerRef:         &featuregate.IssuerReference{Name: "platform", Kind: "ClusterIssuer"},
				CABundleConfigMap: &featuregate.CABundleReference{Name: "platform-trust", Key: "roots.pem"},
			}
		}
		r := NewTLSReconciler(Config{WatchNamespace: "metrics", FeatureGate: featuregate.Config{ServerTLS: &cfg}}, nil, nil)
		for _, tc := range []struct {
			name string
			obj  client.Object
			want bool
		}{
			{"query", &v1alpha1.ThanosQuery{}, false},
			{"receive", &v1alpha1.ThanosReceive{}, false},
			{"store", &v1alpha1.ThanosStore{}, false},
			{"ruler", &v1alpha1.ThanosRuler{}, false},
			{"compact", &v1alpha1.ThanosCompact{}, false},
			{featuregate.TLSCAName, &cmv1.Certificate{}, !external},
			{featuregate.TLSCAName, &corev1.Secret{}, !external},
			{featuregate.TLSCAName, &cmv1.Issuer{}, !external},
			{manifests.TLSBootstrapIssuerName, &cmv1.Issuer{}, !external},
			{cfg.CABundle().Name, &corev1.ConfigMap{}, true},
			{"leaf-tls", &cmv1.Certificate{}, false},
			{"leaf-tls", &corev1.Secret{}, false},
			{"leaf-tls", &corev1.ConfigMap{}, false},
			{"unrelated", &cmv1.Issuer{}, false},
		} {
			tc.obj.SetName(tc.name)
			tc.obj.SetNamespace("unrelated")
			require.Empty(t, r.enqueueNamespace(context.Background(), tc.obj))
			tc.obj.SetNamespace("metrics")
			requests := r.enqueueNamespace(context.Background(), tc.obj)
			if tc.want {
				require.Equal(t, []ctrl.Request{{NamespacedName: client.ObjectKey{Namespace: "metrics", Name: featuregate.TLSCAName}}}, requests)
			} else {
				require.Empty(t, requests, "%T %s, external=%t", tc.obj, tc.name, external)
			}
		}
		for _, leaf := range []client.Object{&cmv1.Certificate{}, &corev1.Secret{}} {
			leaf.SetName("leaf-tls")
			leaf.SetNamespace("metrics")
			leaf.SetLabels(map[string]string{manifests.TLSLabel: tlsManagedValue})
			require.Len(t, r.enqueueNamespace(context.Background(), leaf), 1)
		}

	}
}

func TestTLSReconcileScope(t *testing.T) {
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{v1alpha1.AddToScheme, corev1.AddToScheme, cmv1.AddToScheme} {
		require.NoError(t, add(scheme))
	}
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	flags := featuregate.Flag{featuregate.ServerTLS}
	r := NewTLSReconciler(Config{WatchNamespace: "metrics", FeatureGate: flags.ToFeatureGate()}, c, scheme)
	ctx := context.Background()
	key := client.ObjectKey{Namespace: "unrelated", Name: featuregate.TLSCAName}
	_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	require.NoError(t, err)
	require.True(t, apierrors.IsNotFound(c.Get(ctx, key, &cmv1.Certificate{})))
	key.Namespace = "metrics"
	_, err = r.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	require.ErrorContains(t, err, "waiting for namespace CA", "bootstrap without component resources")
	ca := &cmv1.Certificate{}
	require.NoError(t, c.Get(ctx, key, ca))
	require.True(t, ca.Spec.IsCA)
	require.Empty(t, ca.OwnerReferences)
	require.True(t, apierrors.IsNotFound(c.Get(ctx, key, &corev1.ConfigMap{})))

	query := &v1alpha1.ThanosQuery{ObjectMeta: metav1.ObjectMeta{Name: "query", Namespace: "metrics"}}
	require.NoError(t, c.Create(ctx, query))
	require.NoError(t, c.Delete(ctx, query))
	_, err = r.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	require.ErrorContains(t, err, "waiting for namespace CA", "keep preparing trust after the last component is deleted")
	require.NoError(t, c.Get(ctx, key, ca), "retain existing namespace trust resources")
}

func TestTLSRequiresWatchNamespace(t *testing.T) {
	flags := featuregate.Flag{featuregate.ServerTLS}
	cfg := flags.ToFeatureGate()
	_, err := CacheOptionsForNamespace("", cfg)
	require.ErrorContains(t, err, "server-tls requires --watch-namespace")
	r := NewTLSReconciler(Config{FeatureGate: cfg}, nil, nil)
	require.ErrorContains(t, r.SetupWithManager(nil), "server-tls requires --watch-namespace")
	scoped, err := CacheOptionsForNamespace("metrics", cfg)
	require.NoError(t, err)
	require.Contains(t, scoped.DefaultNamespaces, "metrics")
	_, err = CacheOptionsForNamespace("metrics,other", cfg)
	require.ErrorContains(t, err, "invalid watch namespace")
}

func TestTLSDisabled(t *testing.T) {
	r := NewTLSReconciler(Config{}, nil, nil)
	require.NoError(t, r.SetupWithManager(nil), "disabled TLS must not register watches")
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "metrics", Name: featuregate.TLSCAName}})
	require.NoError(t, err)
}

func newTestTLSReconciler(t *testing.T) *TLSReconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{v1alpha1.AddToScheme, corev1.AddToScheme, cmv1.AddToScheme} {
		require.NoError(t, add(scheme))
	}
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	flags := featuregate.Flag{featuregate.ServerTLS}
	return NewTLSReconciler(Config{WatchNamespace: "test", FeatureGate: flags.ToFeatureGate()}, c, scheme)
}

func reconcileTestTLSNamespace(ctx context.Context, r *TLSReconciler) error {
	_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "test", Name: featuregate.TLSCAName}})
	return err
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
	return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test", Labels: map[string]string{manifests.TLSLabel: "true"}, Annotations: map[string]string{"cert-manager.io/certificate-name": name}}, Data: map[string][]byte{corev1.TLSCertKey: data}}
}

func TestAutomaticCAAndRenewal(t *testing.T) {
	ctx := context.Background()
	r := newTestTLSReconciler(t)
	require.ErrorContains(t, reconcileTestTLSNamespace(ctx, r), "waiting for namespace CA")
	root := rootPEM(t)
	secret := managedSecret(featuregate.TLSCAName, root)
	require.NoError(t, r.Client.Create(ctx, secret))
	require.NoError(t, reconcileTestTLSNamespace(ctx, r))
	bundle := &corev1.ConfigMap{}
	key := client.ObjectKey{Namespace: "test", Name: featuregate.TLSCAName}
	require.NoError(t, r.Client.Get(ctx, key, bundle))
	require.Empty(t, bundle.OwnerReferences)
	require.Contains(t, bundle.Data[featuregate.TLSCAKey], string(root))
	version := bundle.ResourceVersion
	require.NoError(t, reconcileTestTLSNamespace(ctx, r))
	require.NoError(t, r.Client.Get(ctx, key, bundle))
	require.Equal(t, version, bundle.ResourceVersion, "unchanged trust must not trigger updates")
	newRoot := rootPEM(t)
	secret.Data[corev1.TLSCertKey] = newRoot
	require.NoError(t, r.Client.Update(ctx, secret))
	require.NoError(t, reconcileTestTLSNamespace(ctx, r))
	require.NoError(t, r.Client.Get(ctx, key, bundle))
	require.Contains(t, bundle.Data[featuregate.TLSCAKey], string(root))
	require.Contains(t, bundle.Data[featuregate.TLSCAKey], string(newRoot))
}

func TestRefusesUnmanagedResources(t *testing.T) {
	ctx := context.Background()
	r := newTestTLSReconciler(t)
	issuer := &cmv1.Issuer{ObjectMeta: metav1.ObjectMeta{Name: manifests.TLSBootstrapIssuerName, Namespace: "test"}}
	require.NoError(t, r.Client.Create(ctx, issuer))
	require.ErrorContains(t, reconcileTestTLSNamespace(ctx, r), "outside this integration")
	r = newTestTLSReconciler(t)
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: featuregate.TLSCAName, Namespace: "test"}}
	require.NoError(t, r.Client.Create(ctx, secret))
	require.ErrorContains(t, reconcileTestTLSNamespace(ctx, r), "outside this integration")
}

func TestTLSExternalIssuer(t *testing.T) {
	ctx := context.Background()
	r := newTestTLSReconciler(t)
	r.featureGate.ServerTLS.CertManager = &featuregate.CertManagerConfig{IssuerRef: &featuregate.IssuerReference{Name: "platform", Kind: "ClusterIssuer"}, CABundleConfigMap: &featuregate.CABundleReference{Name: "trust", Key: "roots.pem"}}
	bundle := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "trust", Namespace: "test"}, Data: map[string]string{"roots.pem": string(rootPEM(t))}}
	require.NoError(t, r.Client.Create(ctx, bundle))
	require.NoError(t, reconcileTestTLSNamespace(ctx, r))
	issuers := &cmv1.IssuerList{}
	require.NoError(t, r.Client.List(ctx, issuers))
	require.Empty(t, issuers.Items)
}

func newTLSResourceReconciler(t *testing.T) (*TLSReconciler, *handlers.Handler) {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{v1alpha1.AddToScheme, cmv1.AddToScheme, corev1.AddToScheme, appsv1.AddToScheme} {
		require.NoError(t, add(scheme))
	}
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	flags := featuregate.Flag{featuregate.ServerTLS}
	return NewTLSReconciler(Config{WatchNamespace: "test", FeatureGate: flags.ToFeatureGate()}, c, scheme), handlers.NewHandler(c, scheme, logr.Discard())
}

func TestExternalIssuerAndWorkloadLifecycle(t *testing.T) {
	ctx := context.Background()
	r, h := newTLSResourceReconciler(t)
	owner := &v1alpha1.ThanosReceive{ObjectMeta: metav1.ObjectMeta{Name: "receive", Namespace: "test", UID: "cr-uid"}}
	r.featureGate.ServerTLS.CertManager = &featuregate.CertManagerConfig{IssuerRef: &featuregate.IssuerReference{Name: "platform", Kind: "ClusterIssuer"}, CABundleConfigMap: &featuregate.CABundleReference{Name: "trust", Key: "roots.pem"}}
	bundle := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "trust", Namespace: "test"}, Data: map[string]string{"roots.pem": string(rootPEM(t))}}
	require.NoError(t, r.Client.Create(ctx, bundle))
	workload := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "receive", Namespace: "test", UID: "workload-uid"}, Spec: appsv1.StatefulSetSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Args: []string{"receive"}}}}}}}
	resources := manifests.AppendTLSResources([]client.Object{workload}, r.featureGate)
	for _, obj := range resources {
		if cert, ok := obj.(*cmv1.Certificate); ok {
			cert.UID = "certificate-uid"
		}
	}
	require.NoError(t, prepareTLSResources(ctx, r.Client, r.featureGate, owner, resources))
	require.Zero(t, h.CreateOrUpdate(ctx, owner.Namespace, owner, resources))
	cert := &cmv1.Certificate{}
	key := client.ObjectKey{Name: manifests.TLSResourceName(workload.Name), Namespace: "test"}
	require.NoError(t, r.Client.Get(ctx, key, cert))
	require.True(t, metav1.IsControlledBy(cert, owner))
	web := &corev1.ConfigMap{}
	require.NoError(t, r.Client.Get(ctx, key, web))
	require.True(t, metav1.IsControlledBy(web, owner))

	require.Equal(t, "ClusterIssuer", cert.Spec.IssuerRef.Kind)
	require.Equal(t, []string{"receive.test.svc", "*.receive.test.svc"}, cert.Spec.DNSNames)
	require.Equal(t, []cmv1.KeyUsage{cmv1.UsageDigitalSignature, cmv1.UsageServerAuth}, cert.Spec.Usages)
	require.NoError(t, prepareTLSResources(ctx, r.Client, r.featureGate, owner, resources))
	initialTemplate := workload.Spec.Template.DeepCopy()
	first := initialTemplate.Annotations[manifests.TLSTrustChecksumAnnotation]
	require.NotEmpty(t, first, "trust checksum must exist before leaf issuance")
	secret := managedSecret(key.Name, []byte("certificate one"))
	require.NoError(t, r.Client.Create(ctx, secret))
	require.NoError(t, prepareTLSResources(ctx, r.Client, r.featureGate, owner, resources))
	require.Zero(t, h.CreateOrUpdate(ctx, owner.Namespace, owner, resources))
	require.NoError(t, prepareTLSResources(ctx, r.Client, r.featureGate, owner, resources))
	require.Equal(t, *initialTemplate, workload.Spec.Template, "initial issuance must not roll pods")
	require.NoError(t, prepareTLSResources(ctx, r.Client, r.featureGate, owner, resources))
	require.Equal(t, *initialTemplate, workload.Spec.Template, "unchanged trust must keep the template stable")
	require.NoError(t, r.syncCertificateSecrets(ctx))
	require.NoError(t, r.Client.Get(ctx, key, secret))
	require.True(t, metav1.IsControlledBy(secret, cert))
	secret.Data[corev1.TLSCertKey] = []byte("certificate two")
	secret.Data[corev1.TLSPrivateKeyKey] = []byte("rotated private key")
	require.NoError(t, r.Client.Update(ctx, secret))
	require.NoError(t, prepareTLSResources(ctx, r.Client, r.featureGate, owner, resources))
	require.Equal(t, *initialTemplate, workload.Spec.Template, "leaf renewal must not roll pods")
	bundle.Data["roots.pem"] += string(rootPEM(t))
	require.NoError(t, r.Client.Update(ctx, bundle))
	require.NoError(t, prepareTLSResources(ctx, r.Client, r.featureGate, owner, resources))
	require.NotEqual(t, first, workload.Spec.Template.Annotations[manifests.TLSTrustChecksumAnnotation], "trust changes must refresh clients")
	require.Zero(t, h.NewResourcePruner().WithCertificate().WithConfigMap().
		PruneByOwner(ctx, owner, client.MatchingLabels{manifests.TLSLabel: "true"}))
	// The fake client does not run garbage collection; the Secret follows its Certificate.
	require.NoError(t, r.Client.Get(ctx, key, secret))
	require.True(t, metav1.IsControlledBy(secret, cert))
	require.True(t, apierrors.IsNotFound(r.Client.Get(ctx, key, &cmv1.Certificate{})))
	require.True(t, apierrors.IsNotFound(r.Client.Get(ctx, key, &corev1.ConfigMap{})))
	require.NoError(t, r.Client.Get(ctx, client.ObjectKeyFromObject(bundle), &corev1.ConfigMap{}), "external trust must survive disable")
}

func TestTrustChecksumRequiresValidBundle(t *testing.T) {
	ctx := context.Background()
	r, _ := newTLSResourceReconciler(t)
	owner := &v1alpha1.ThanosReceive{ObjectMeta: metav1.ObjectMeta{Name: "receive", Namespace: "test", UID: "cr-uid"}}
	workload := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "query", Namespace: "test"},
		Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Args: []string{"query"}}},
		}}},
	}
	initial := workload.Spec.Template.DeepCopy()
	resources := []client.Object{workload}
	require.Error(t, prepareTLSResources(ctx, r.Client, r.featureGate, owner, resources))
	bundle := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: featuregate.TLSCAName, Namespace: "test"}}
	require.NoError(t, r.Client.Create(ctx, bundle))
	for _, content := range []string{"", "not PEM"} {
		bundle.Data = map[string]string{featuregate.TLSCAKey: content}
		require.NoError(t, r.Client.Update(ctx, bundle))
		require.Error(t, prepareTLSResources(ctx, r.Client, r.featureGate, owner, resources))
		require.Equal(t, *initial, workload.Spec.Template)
	}
	bundle.Data[featuregate.TLSCAKey] = string(rootPEM(t))
	require.NoError(t, r.Client.Update(ctx, bundle))
	require.NoError(t, prepareTLSResources(ctx, r.Client, r.featureGate, owner, resources))
	require.NotEmpty(t, workload.Spec.Template.Annotations[manifests.TLSTrustChecksumAnnotation])
}

func TestTLSShardPruning(t *testing.T) {
	ctx := context.Background()
	tls, h := newTLSResourceReconciler(t)
	owner := &v1alpha1.ThanosStore{ObjectMeta: metav1.ObjectMeta{Name: "store", Namespace: "test", UID: "store-uid"}}
	opts := manifestsstore.Options{Options: manifests.Options{Owner: owner.Name}}
	labels := manifests.MergeMaps(opts.GetSelectorLabels(), map[string]string{manifests.TLSLabel: "true"})
	for _, name := range []string{"kept", "removed"} {
		objects := []client.Object{
			&cmv1.Certificate{ObjectMeta: metav1.ObjectMeta{Name: manifests.TLSResourceName(name), Labels: labels}},
			&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: manifests.TLSResourceName(name), Labels: labels}},
		}
		require.Zero(t, h.CreateOrUpdate(ctx, owner.Namespace, owner, objects))
	}
	shared := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: featuregate.TLSCAName, Namespace: owner.Namespace, Labels: map[string]string{manifests.TLSLabel: "true"}}}
	require.NoError(t, tls.Create(ctx, shared))
	r := &ThanosStoreReconciler{Client: tls.Client, handler: h, featureGate: tls.featureGate}
	require.Zero(t, r.pruneOrphanedResources(ctx, owner.Namespace, owner.Name, []string{"kept"}))
	for _, obj := range []client.Object{&cmv1.Certificate{}, &corev1.ConfigMap{}} {
		require.NoError(t, tls.Get(ctx, client.ObjectKey{Namespace: owner.Namespace, Name: "kept-tls"}, obj))
		require.True(t, apierrors.IsNotFound(tls.Get(ctx, client.ObjectKey{Namespace: owner.Namespace, Name: "removed-tls"}, obj)))
	}
	require.NoError(t, tls.Get(ctx, client.ObjectKeyFromObject(shared), shared))
}
