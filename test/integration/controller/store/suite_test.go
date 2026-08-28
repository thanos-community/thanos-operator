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

// Package store runs the ThanosStore controller's envtest specs in their own test
// binary. Every controller suite boots suite.Setup, which registers all five
// reconcilers on one manager (so the "all controllers coexist on one manager"
// coverage is retained), then exercises only this controller's specs. Splitting
// per controller lets the suites run as parallel binaries under go test and keeps
// one controller's failure from aborting the others.
package store

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/test/integration/suite"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	k8sClient client.Client
	env       *suite.Env
	ctx       context.Context
	cancel    context.CancelFunc
)

// skipValue matches the EXCLUDE_* env-var opt-out convention the specs use to skip
// this controller's tests.
const skipValue = "true"

func TestThanosStore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ThanosStore Controller Suite")
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
