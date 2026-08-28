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

// Package controller holds the core cross-controller envtest suite. Unlike the
// isolated behavioral suites (feature gates, PDB, pause), which each boot a manager
// with a specific configuration, this suite runs all five controllers on a single
// shared manager -- the cross-controller service discovery and watch coupling
// exercised here is the integration coverage. It is black-box: it lives outside
// internal/controller and drives the controllers through their public surface only.
package controller

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/test/integration/suite"

	"sigs.k8s.io/controller-runtime/pkg/client"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	k8sClient client.Client
	env       *suite.Env
	ctx       context.Context
	cancel    context.CancelFunc
)

// skipValue matches the EXCLUDE_* env-var opt-out convention the specs use to skip a
// controller's tests.
const skipValue = "true"

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Core Controller Suite")
}

var _ = BeforeSuite(func() {
	By("bootstrapping test environment")
	// suite.Setup registers all five controllers on one manager with the
	// operator's default configuration (all feature gates off) -- the gated
	// behaviors are covered by the matching suites under test/integration.
	env, ctx, cancel = suite.Setup(featuregate.Config{})
	k8sClient = env.Client
	Expect(k8sClient).NotTo(BeNil())

	//+kubebuilder:scaffold:scheme
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	Expect(env.Stop()).To(Succeed())
})
