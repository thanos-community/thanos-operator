// Package coretls runs the core data-flow scenarios and TLS lifecycle checks.
// Each ordered group owns a namespace and operator so migration stays isolated.
package coretls

import (
	"context"
	"testing"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/thanos-community/thanos-operator/test/e2e/suite"
)

var c client.Client
var ctx = context.Background()

func TestCoreTLS(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Thanos Core TLS E2E Suite")
}

var _ = BeforeSuite(func() {
	v, err := version.ParseSemantic(*suite.ThanosVersion())
	Expect(err).NotTo(HaveOccurred())
	if v.LessThan(version.MustParseSemantic("v0.42.0")) {
		Skip("TLS requires Thanos v0.42.0 or newer")
	}
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	c = suite.NewClient()
	Expect(cmv1.AddToScheme(c.Scheme())).To(Succeed())
})
