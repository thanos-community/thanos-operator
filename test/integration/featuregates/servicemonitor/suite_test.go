package servicemonitor

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/test/integration/suite"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// This suite runs the operator with only the service-monitor feature gate enabled
// and asserts that every component produces its ServiceMonitor. It exercises the
// pure (gate -> manifest) behavior only; SD and cross-controller watches are the
// core ordered suite's concern.

var (
	k8sClient client.Client
	env       *suite.Env
	ctx       context.Context
	cancel    context.CancelFunc
)

func TestServiceMonitorGate(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ServiceMonitor FeatureGate Suite")
}

var _ = BeforeSuite(func() {
	env, ctx, cancel = suite.Setup(featuregate.Config{EnableServiceMonitor: true})
	k8sClient = env.Client
	Expect(k8sClient).NotTo(BeNil())
})

var _ = AfterSuite(func() {
	cancel()
	Expect(env.Stop()).To(Succeed())
})
