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

// Package status runs envtest specs for ObjectStatusReconciler and the shared
// status patch helpers used by the component controllers.
package status

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

func TestObjectStatus(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ObjectStatus Controller Suite")
}

var _ = BeforeSuite(func() {
	// Condition updates must stay enabled so we can verify merge patches do not
	// clobber each other across the two status writers.
	env, ctx, cancel = suite.Setup(featuregate.Config{}, suite.WithConditionUpdates())
	k8sClient = env.Client
	Expect(k8sClient).NotTo(BeNil())
})

var _ = AfterSuite(func() {
	cancel()
	Expect(env.Stop()).To(Succeed())
})
