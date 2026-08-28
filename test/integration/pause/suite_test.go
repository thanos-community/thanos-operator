package pause

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/test/integration/suite"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// This suite checks Spec.Paused in isolation. Honoring pause is per-CR and
// per-reconciler (it never touches the cross-controller watches), so it is
// order-independent and lives here rather than in the core ordered suite. All
// five components share one namespace: they are created, allowed to reconcile,
// then paused together and mutated, and a single Consistently window proves none
// of them reconciled the change. That collapses what used to be five separate
// per-controller Consistently waits into one, and covers Compact and Ruler,
// which had no pause coverage before. Pause is not feature gated, so the suite
// runs with the operator default (all gates off).

var (
	k8sClient client.Client
	env       *suite.Env
	ctx       context.Context
	cancel    context.CancelFunc
)

func TestPause(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pause Suite")
}

var _ = BeforeSuite(func() {
	env, ctx, cancel = suite.Setup(featuregate.Config{})
	k8sClient = env.Client
	Expect(k8sClient).NotTo(BeNil())
})

var _ = AfterSuite(func() {
	cancel()
	Expect(env.Stop()).To(Succeed())
})
