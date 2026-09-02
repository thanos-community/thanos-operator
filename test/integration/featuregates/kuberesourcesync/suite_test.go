package kuberesourcesync

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/test/integration/suite"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// This suite checks the kube-resource-sync sidecar in isolation. With the gate
// on, the ThanosReceive router gains a sync sidecar plus init container, an
// EmptyDir hashring volume, and a Role/RoleBinding letting it read ConfigMaps.
// The behavior is reconciler-global, so it runs in its own binary with the gate
// enabled. Only the receive router is affected.

const kubeResourceSyncImage = "quay.io/philipgough/kube-resource-sync:test"

var (
	k8sClient client.Client
	env       *suite.Env
	ctx       context.Context
	cancel    context.CancelFunc
)

func TestKubeResourceSync(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "KubeResourceSync FeatureGate Suite")
}

var _ = BeforeSuite(func() {
	env, ctx, cancel = suite.Setup(featuregate.Config{
		KubeResourceSync: &featuregate.KubeResourceSyncConfig{
			FeatureConfig: featuregate.FeatureConfig{Enabled: true},
			Image:         kubeResourceSyncImage,
		},
	})
	k8sClient = env.Client
	Expect(k8sClient).NotTo(BeNil())
})

var _ = AfterSuite(func() {
	cancel()
	Expect(env.Stop()).To(Succeed())
})
