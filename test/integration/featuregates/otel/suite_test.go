package otel

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/test/integration"
	"github.com/thanos-community/thanos-operator/test/integration/testenv"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// This suite checks OpenTelemetry sidecar injection in isolation. With the gate
// on, every component's pod template gets the inject annotation and a
// --tracing.config arg on its main container. The behavior is reconciler-global
// (set once at construction), so it runs in its own binary with the gate enabled.
// Each component is exercised in its own namespace so the checks are independent
// and order-free.

var (
	k8sClient client.Client
	env       *testenv.Env
	ctx       context.Context
	cancel    context.CancelFunc
)

func TestOtelSidecar(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OtelSidecar FeatureGate Suite")
}

var _ = BeforeSuite(func() {
	env, ctx, cancel = integration.Setup(featuregate.Config{EnableOtelSidecar: true})
	k8sClient = env.Client
	Expect(k8sClient).NotTo(BeNil())
})

var _ = AfterSuite(func() {
	cancel()
	Expect(env.Stop()).To(Succeed())
})
