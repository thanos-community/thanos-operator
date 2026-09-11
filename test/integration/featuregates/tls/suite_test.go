package tls

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/test/integration/suite"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	k8sClient client.Client
	env       *suite.Env
	ctx       context.Context
	cancel    context.CancelFunc
)

func TestTLSGate(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TLS FeatureGate Suite")
}

var _ = BeforeSuite(func() {
	// ServiceMonitor is enabled to check the TLS settings on generated scrapes.
	flags := featuregate.Flag{featuregate.TLS, featuregate.ServiceMonitor}
	env, ctx, cancel = suite.Setup(flags.ToFeatureGate())
	k8sClient = env.Client
})

var _ = AfterSuite(func() {
	if cancel != nil {
		cancel()
	}
	if env != nil {
		Expect(env.Stop()).To(Succeed())
	}
})
