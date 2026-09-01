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

// Command setup performs the one-time cluster bootstrap the e2e suites share:
// prometheus-operator, cert-manager, MinIO + its object-storage secret, and the
// test Prometheus. It runs once (via `make e2e-setup`) before the suites, so the
// per-suite test binaries can run concurrently against one cluster without racing
// on this shared setup. Operator image build/load and deploy are handled by the
// make target around this command.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	"github.com/thanos-community/thanos-operator/test/utils"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const operatorNamespace = "thanos-operator-system"

func main() {
	log.SetFlags(0)
	if err := run(); err != nil {
		log.Fatalf("e2e setup failed: %v", err)
	}
	log.Println(">> e2e setup complete")
}

func run() error {
	image := flag.String("image", "", "operator image to load into the kind cluster (skips load if empty)")
	flag.Parse()

	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		corev1.AddToScheme, rbacv1.AddToScheme, monitoringv1.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			return fmt.Errorf("building scheme: %w", err)
		}
	}
	c, err := client.New(config.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	log.Println(">> installing prometheus-operator")
	if err := utils.InstallPrometheusOperator(); err != nil {
		return fmt.Errorf("installing prometheus-operator: %w", err)
	}
	log.Println(">> installing cert-manager")
	if err := utils.InstallCertManager(); err != nil {
		return fmt.Errorf("installing cert-manager: %w", err)
	}
	log.Println(">> creating operator namespace")
	if err := ensureNamespace(c, operatorNamespace); err != nil {
		return fmt.Errorf("creating namespace %s: %w", operatorNamespace, err)
	}
	log.Println(">> installing minio")
	if err := utils.InstallMinIO(); err != nil {
		return fmt.Errorf("installing minio: %w", err)
	}
	log.Println(">> creating object storage secret")
	if err := utils.CreateMinioObjectStorageSecret(); err != nil {
		return fmt.Errorf("creating object storage secret: %w", err)
	}
	log.Println(">> setting up prometheus")
	if err := utils.SetUpPrometheus(c); err != nil {
		return fmt.Errorf("setting up prometheus: %w", err)
	}
	if *image != "" {
		log.Printf(">> loading operator image %s into kind", *image)
		if err := utils.LoadImageToKindClusterWithName(*image); err != nil {
			return fmt.Errorf("loading image into kind: %w", err)
		}
	}
	return nil
}

func ensureNamespace(c client.Client, name string) error {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	if err := c.Create(context.Background(), ns); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}
