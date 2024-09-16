package manifests

import (
	"fmt"
	"reflect"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	"github.com/imdario/mergo"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// MutateFuncFor returns a mutate function based on the existing resource's concrete type.
// It currently supports the following types and will return an error for other types:
//
//   - ConfigMap
//   - Secret
//   - Service
//   - ServiceAccount
//   - Deployment
//   - StatefulSet
//   - ServiceMonitor
func MutateFuncFor(existing, desired client.Object) controllerutil.MutateFn {
	return func() error {
		existingAnnotations := existing.GetAnnotations()
		if err := mergeWithOverride(&existingAnnotations, desired.GetAnnotations()); err != nil {
			return err
		}
		existing.SetAnnotations(existingAnnotations)

		existingLabels := existing.GetLabels()
		if err := mergeWithOverride(&existingLabels, desired.GetLabels()); err != nil {
			return err
		}
		existing.SetLabels(existingLabels)

		if ownerRefs := desired.GetOwnerReferences(); len(ownerRefs) > 0 {
			existing.SetOwnerReferences(ownerRefs)
		}

		switch existing.(type) {
		case *corev1.ConfigMap:
			cm := existing.(*corev1.ConfigMap)
			wantCm := desired.(*corev1.ConfigMap)
			mutateConfigMap(cm, wantCm)

		case *corev1.Secret:
			s := existing.(*corev1.Secret)
			wantS := desired.(*corev1.Secret)
			mutateSecret(s, wantS)
			existingAnnotations := s.GetAnnotations()
			s.SetAnnotations(existingAnnotations)

		case *corev1.Service:
			svc := existing.(*corev1.Service)
			wantSvc := desired.(*corev1.Service)
			mutateService(svc, wantSvc)

		case *corev1.ServiceAccount:
			sa := existing.(*corev1.ServiceAccount)
			wantSa := desired.(*corev1.ServiceAccount)
			mutateServiceAccount(sa, wantSa)

		case *appsv1.Deployment:
			dpl := existing.(*appsv1.Deployment)
			wantDpl := desired.(*appsv1.Deployment)
			mutateDeployment(dpl, wantDpl)

		case *appsv1.StatefulSet:
			sts := existing.(*appsv1.StatefulSet)
			wantSts := desired.(*appsv1.StatefulSet)
			mutateStatefulSet(sts, wantSts)

		case *monitoringv1.ServiceMonitor:
			sm := existing.(*monitoringv1.ServiceMonitor)
			wantSm := desired.(*monitoringv1.ServiceMonitor)
			mutateServiceMonitor(sm, wantSm)

		default:
			t := reflect.TypeOf(existing).String()
			return fmt.Errorf("missing mutate implementation for resource type %v", t)
		}
		return nil
	}
}

func mergeWithOverride(dst, src interface{}) error {
	err := mergo.Merge(dst, src, mergo.WithOverride)
	if err != nil {
		return fmt.Errorf("unable to mergeWithOverride: %w", err)
	}
	return nil
}

func mutateConfigMap(existing, desired *corev1.ConfigMap) {
	existing.Annotations = desired.Annotations
	existing.Labels = desired.Labels
	existing.BinaryData = desired.BinaryData
	existing.Data = desired.Data
}

func mutateSecret(existing, desired *corev1.Secret) {
	existing.Annotations = desired.Annotations
	existing.Labels = desired.Labels
	existing.Data = desired.Data
}

func mutateServiceAccount(existing, desired *corev1.ServiceAccount) {
	// Do not add annotations to ServiceAccount to avoid spawning Secrets
	// Values in annotations will be injected by platforms like OCP
	// https://kubernetes.io/docs/reference/access-authn-authz/service-accounts-admin/#auto-generated-legacy-serviceaccount-token-clean-up
	existing.Labels = desired.Labels
}

func mutateService(existing, desired *corev1.Service) {
	existing.Spec.Ports = desired.Spec.Ports
	existing.Spec.Selector = desired.Spec.Selector
	existing.Labels = desired.Labels
}

func mutateDeployment(existing, desired *appsv1.Deployment) {
	// Deployment selector is immutable so we set this value only if
	// a new object is going to be created
	if existing.CreationTimestamp.IsZero() {
		existing.Spec.Selector = desired.Spec.Selector
	}
	existing.Spec.Replicas = desired.Spec.Replicas
	existing.Spec.Strategy = desired.Spec.Strategy
	mutatePodTemplate(&existing.Spec.Template, &desired.Spec.Template)
}

func mutateStatefulSet(existing, desired *appsv1.StatefulSet) {
	// StatefulSet selector is immutable so we set this value only if
	// a new object is going to be created
	if existing.CreationTimestamp.IsZero() {
		existing.Spec.Selector = desired.Spec.Selector
	}
	existing.Spec.Replicas = desired.Spec.Replicas
	mutatePodTemplate(&existing.Spec.Template, &desired.Spec.Template)
}

func mutatePodTemplate(existing, desired *corev1.PodTemplateSpec) {
	existing.Annotations = desired.Annotations
	existing.Labels = desired.Labels
	mutatePodSpec(&existing.Spec, &desired.Spec)
}

func mutatePodSpec(existing *corev1.PodSpec, desired *corev1.PodSpec) {
	existing.Affinity = desired.Affinity
	existing.Containers = desired.Containers
	existing.InitContainers = desired.InitContainers
	existing.NodeSelector = desired.NodeSelector
	existing.Tolerations = desired.Tolerations
	existing.TopologySpreadConstraints = desired.TopologySpreadConstraints
	existing.Volumes = desired.Volumes
}

func mutateServiceMonitor(existing, desired *monitoringv1.ServiceMonitor) {
	existing.Labels = desired.Labels
	existing.Spec.Selector.MatchLabels = desired.Spec.Selector.MatchLabels
	existing.Spec.NamespaceSelector = desired.Spec.NamespaceSelector
	existing.Spec.Endpoints = desired.Spec.Endpoints
}
