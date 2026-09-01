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

// Package stateless runs the stateless-ruler tenancy feature as its own e2e test
// binary against the shared cluster bootstrapped by `make e2e-setup`. Unlike the
// leaf suites (compact, capnproto receive), the stateless ruler is not self-contained
// on the operator alone: discovery is same-namespace, so it stands up its own receive
// and query (via the suite fixtures) and exercises the full remote-write -> evaluate
// -> read-back path in namespace e2e-ruler-stateless. It is grouped under test/e2e/ruler
// so further ruler feature packages can sit beside it and run in parallel.
package stateless

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thanos-community/thanos-operator/test/e2e/suite"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const namespace = "e2e-ruler-stateless"

var (
	c        client.Client
	teardown func()
)

func TestStatelessRuler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ThanosRuler Stateless E2E Suite")
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
