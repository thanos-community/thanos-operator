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

// Package capnproto runs the capnproto-replication ThanosReceive variant as its own
// test binary against the shared cluster bootstrapped by `make e2e-setup`. It is a
// self-contained leaf: it creates its own receive, verifies the config/args, and
// remote-writes to it directly, so nothing in the ordered core suite reads back from
// it. The main receive stays in the core suite because the query/ruler data flow
// depends on it. `receive/` is a grouping dir so further self-contained receive
// variants sit beside capnproto and run in parallel.
package capnproto

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thanos-community/thanos-operator/test/e2e/suite"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const namespace = "e2e-receive-capnproto"

var (
	c        client.Client
	teardown func()
)

func TestCapnproto(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ThanosReceive Capnproto E2E Suite")
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
