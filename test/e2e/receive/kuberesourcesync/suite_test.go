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

// Package kuberesourcesync runs the kube-resource-sync feature-gate e2e specs for the
// receive router as its own test binary against the shared cluster bootstrapped by
// `make e2e-setup` (which enables the feature on the operator). Unlike the envtest
// integration suite, which can only inspect the rendered Deployment, this exercises the
// feature for real: the router pod must come up with the injected init container and
// sidecar, and remote-write must route through the hashring the sidecar syncs into the
// shared EmptyDir. It is a self-contained leaf in namespace e2e-receive-krs.
package kuberesourcesync

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thanos-community/thanos-operator/test/e2e/suite"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const namespace = "e2e-receive-krs"

var (
	c        client.Client
	teardown func()
)

func TestKubeResourceSync(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ThanosReceive KubeResourceSync E2E Suite")
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
