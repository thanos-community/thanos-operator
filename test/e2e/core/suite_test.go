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

// Package core runs the ordered receive -> query -> ruler data-flow suite that is the
// heart of the e2e coverage: a remote-write lands in the receiver, a query reads it
// back, and a ruler evaluates against it. These specs must run in order, so they live
// in one binary rather than being split per area like the leaf feature suites. Like
// those suites it creates its own throwaway namespace and tears it down in AfterSuite,
// so reruns are idempotent and nothing leaks into the shared operator namespace. The
// cluster (operator, prometheus-operator, cert-manager, MinIO, test Prometheus) is
// bootstrapped once by `make e2e-setup`.
package core

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thanos-community/thanos-operator/test/e2e/suite"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	// namespace is the dedicated, throwaway namespace this suite creates and tears
	// down, so reruns are idempotent and nothing leaks into the shared operator ns.
	namespace = "e2e-core"
	// operatorNamespace is where `make e2e-setup` deploys the operator. The suite only
	// reads the controller-manager deployment there; it never modifies it.
	operatorNamespace = "thanos-operator-system"
)

var (
	c        client.Client
	teardown func()
)

func TestCore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ThanosCore E2E Suite")
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
