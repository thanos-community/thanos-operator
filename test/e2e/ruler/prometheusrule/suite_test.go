/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package prometheusrule runs the PrometheusRule feature-gate e2e as its own test
// binary against the shared cluster bootstrapped by `make e2e-setup` (the operator is
// deployed with --enable-feature=prometheus-rule). It stands up its own query so the
// ruler can reconcile, then proves a PrometheusRule-derived rule is wired into a real
// ruler pod and evaluates end-to-end. It sits beside ruler/stateless under
// test/e2e/ruler and runs in parallel with it.
package prometheusrule

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thanos-community/thanos-operator/test/e2e/suite"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const namespace = "e2e-ruler-prometheusrule"

var (
	c        client.Client
	teardown func()
)

func TestPrometheusRuleFeatureGate(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ThanosRuler PrometheusRule Feature Gate E2E Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	c, _, teardown = suite.Setup(namespace)
	Expect(c).NotTo(BeNil())
})

var _ = AfterSuite(func() {
	if teardown != nil {
		teardown()
	}
})
