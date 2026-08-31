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

// Package store runs the ThanosStore e2e specs in their own test binary against the
// shared cluster bootstrapped by `make e2e-setup`. It is a leaf: it needs only the
// operator and object storage, reads no data back, and asserts the store's own shape --
// sharding + HA shards come up, the relabel-config is applied, each shard service
// carries the store-api group-discovery labels, and a scale-down prunes the removed
// shard. Whether a query then discovers those labelled services is the query's concern
// and is covered by the integration suite. It runs in parallel with the other suites in
// namespace e2e-store.
package store

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thanos-community/thanos-operator/test/e2e/suite"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const namespace = "e2e-store"

var (
	c        client.Client
	teardown func()
)

func TestStore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ThanosStore E2E Suite")
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
