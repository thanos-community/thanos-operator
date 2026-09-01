package pdb

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/test/integration/suite"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// This suite checks PodDisruptionBudget behavior in isolation. A PDB is created
// off the replica count and removed when a workload scales to a single replica.
// This is not feature gated, so the suite runs with the operator default (all
// gates off). Each component is exercised in its own namespace so the checks are
// independent and order-free.

var (
	k8sClient client.Client
	env       *suite.Env
	ctx       context.Context
	cancel    context.CancelFunc
)

func TestPodDisruptionBudget(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PodDisruptionBudget Suite")
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
