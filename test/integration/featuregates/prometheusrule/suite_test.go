package prometheusrule

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/test/integration/suite"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// This suite covers PrometheusRule discovery, which is guarded by the
// EnablePrometheusRuleDiscovery feature gate. The gate is reconciler-global, so
// the coverage lives here with the gate turned on rather than in the core suite,
// which runs with the operator default (all gates off). Each spec runs in its own
// namespace so the checks are independent and order-free.

var (
	k8sClient client.Client
	env       *suite.Env
	ctx       context.Context
	cancel    context.CancelFunc
)

func TestPrometheusRuleGate(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PrometheusRule FeatureGate Suite")
}

var _ = BeforeSuite(func() {
	env, ctx, cancel = suite.Setup(featuregate.Config{PrometheusRule: featuregate.Enabled()})
	k8sClient = env.Client
	Expect(k8sClient).NotTo(BeNil())
})

var _ = AfterSuite(func() {
	cancel()
	Expect(env.Stop()).To(Succeed())
})
