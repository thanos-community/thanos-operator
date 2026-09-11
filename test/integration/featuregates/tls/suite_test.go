package tls

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/thanos-community/thanos-operator/test/integration/suite"
	"k8s.io/client-go/kubernetes/scheme"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	k8sClient client.Client
	env       *suite.Env
	ctx       context.Context
)

func TestTLSGate(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TLS FeatureGate Suite")
}

var _ = BeforeSuite(func() {
	ctx = context.Background()
	Expect(cmv1.AddToScheme(scheme.Scheme)).To(Succeed())
	var err error
	env, err = suite.Start("",
		"../../../../config/crd/bases",
		"../../configs/service-monitor.yaml",
		"../../configs/prometheus-rule.yaml",
		"../../configs/cert-manager.yaml",
	)
	Expect(err).NotTo(HaveOccurred())
	k8sClient = env.Client
})

var _ = AfterSuite(func() {
	if env != nil {
		Expect(env.Stop()).To(Succeed())
	}
})
