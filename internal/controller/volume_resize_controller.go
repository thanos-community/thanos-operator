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

package controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// VolumeResizeReconciler reconciles volume resize operations for StatefulSets created by thanos operator
type VolumeResizeReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	logger   logr.Logger
	recorder events.EventRecorder
}

// NewVolumeResizeReconciler returns a reconciler for volume resize operations.
func NewVolumeResizeReconciler(conf Config, client client.Client, scheme *runtime.Scheme) *VolumeResizeReconciler {
	return &VolumeResizeReconciler{
		Client:   client,
		Scheme:   scheme,
		logger:   conf.InstrumentationConfig.Logger.WithName("volume-resize"),
		recorder: conf.InstrumentationConfig.EventRecorder,
	}
}

//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete

// Reconcile handles volume resize operations for StatefulSets managed by thanos operator
func (r *VolumeResizeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	sts := &appsv1.StatefulSet{}
	err := r.Get(ctx, req.NamespacedName, sts)
	if err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Info("StatefulSet not found, ignoring since object may be deleted", "statefulset", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		r.logger.Error(err, "failed to get StatefulSet", "statefulset", req.NamespacedName)
		return ctrl.Result{}, err
	}

	if !r.isThanosOperatorStatefulSet(sts) {
		r.logger.V(1).Info("StatefulSet not managed by thanos operator, skipping", "statefulset", req.NamespacedName)
		return ctrl.Result{}, nil
	}

	r.logger.Info("Processing StatefulSet for volume resize operations", "statefulset", req.NamespacedName)

	err = r.processPVCs(ctx, sts)
	if err != nil {
		r.logger.Error(err, "failed to process PVCs for StatefulSet", "statefulset", req.NamespacedName)
		return ctrl.Result{}, err
	}

	r.recorder.Eventf(sts, nil, corev1.EventTypeNormal, "VolumeResizeProcessed", "Reconcile",
		"Successfully processed volume resize for StatefulSet")

	return ctrl.Result{}, nil
}

// isThanosOperatorStatefulSet checks if a StatefulSet is managed by thanos operator
func (r *VolumeResizeReconciler) isThanosOperatorStatefulSet(sts *appsv1.StatefulSet) bool {
	labels := sts.GetLabels()
	if labels == nil {
		return false
	}

	// Check for thanos operator managed-by label
	managedBy, exists := labels[manifests.ManagedByLabel]
	if !exists || managedBy != manifests.DefaultManagedByLabel {
		return false
	}

	// Check for part-of label to ensure it's a thanos component
	partOf, exists := labels[manifests.PartOfLabel]
	if !exists || partOf != manifests.DefaultPartOfLabel {
		return false
	}

	return true
}

// processPVCs handles the volume resize logic for PVCs associated with the StatefulSet
func (r *VolumeResizeReconciler) processPVCs(ctx context.Context, sts *appsv1.StatefulSet) error {
	pvcList := &corev1.PersistentVolumeClaimList{}
	// Create a selector to find PVCs that belong to this StatefulSet
	labelSelector := client.MatchingLabels{
		"app.kubernetes.io/instance": sts.GetLabels()["app.kubernetes.io/instance"],
	}

	err := r.List(ctx, pvcList, client.InNamespace(sts.GetNamespace()), labelSelector)
	if err != nil {
		return fmt.Errorf("failed to list PVCs for StatefulSet %s: %w", sts.GetName(), err)
	}

	r.logger.Info("Found PVCs for StatefulSet", "statefulset", sts.GetName(), "pvcCount", len(pvcList.Items))

	// Process each PVC
	for _, pvc := range pvcList.Items {
		err = r.processPVC(ctx, &pvc, sts)
		if err != nil {
			r.logger.Error(err, "failed to process PVC", "pvc", pvc.GetName(), "statefulset", sts.GetName())
			// Continue processing other PVCs even if one fails
			continue
		}
	}

	return nil
}

// processPVC handles volume resize operations for a single PVC
func (r *VolumeResizeReconciler) processPVC(ctx context.Context, pvc *corev1.PersistentVolumeClaim, sts *appsv1.StatefulSet) error {
	r.logger.Info("Processing PVC", "pvc", pvc.GetName(), "namespace", pvc.GetNamespace())

	requestedSizeStr, exists := sts.GetAnnotations()[manifests.StorageSizeAnnotation]
	if !exists {
		r.logger.V(1).Info("StatefulSet does not have storage size annotation, skipping", "statefulset", sts.GetName())
		return nil
	}

	requestedSize, err := parseStorageSize(requestedSizeStr)
	if err != nil {
		return fmt.Errorf("failed to parse requested storage size %s for PVC %s: %w", requestedSizeStr, pvc.GetName(), err)
	}

	currentSize := pvc.Spec.Resources.Requests[corev1.ResourceStorage]

	if requestedSize.Cmp(currentSize) > 0 {
		r.logger.Info("PVC needs to be resized",
			"pvc", pvc.GetName(),
			"currentSize", currentSize.String(),
			"requestedSize", requestedSize.String())

		err = r.resizePVC(ctx, pvc, requestedSize)
		if err != nil {
			return fmt.Errorf("failed to resize PVC %s: %w", pvc.GetName(), err)
		}

		r.recorder.Eventf(pvc, nil, corev1.EventTypeNormal, "VolumeResized", "VolumeResize",
			"PVC resized from %s to %s", currentSize.String(), requestedSize.String())
	} else {
		r.logger.V(1).Info("PVC does not need resizing", "pvc", pvc.GetName())
	}

	return nil
}

// resizePVC performs the actual PVC resize operation
func (r *VolumeResizeReconciler) resizePVC(ctx context.Context, pvc *corev1.PersistentVolumeClaim, newSize resource.Quantity) error {
	patch := pvc.DeepCopy()
	patch.Spec.Resources.Requests[corev1.ResourceStorage] = newSize
	err := r.Patch(ctx, patch, client.MergeFrom(pvc))
	if err != nil {
		return fmt.Errorf("failed to patch PVC: %w", err)
	}

	r.logger.Info("Successfully resized PVC", "pvc", pvc.GetName(), "newSize", newSize.String())
	return nil
}

// parseStorageSize parses a storage size string into a resource.Quantity
func parseStorageSize(sizeStr string) (resource.Quantity, error) {
	quantity, err := resource.ParseQuantity(sizeStr)
	if err != nil {
		return resource.Quantity{}, fmt.Errorf("invalid storage size format: %w", err)
	}
	return quantity, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VolumeResizeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create a predicate to only watch StatefulSets managed by thanos operator
	thanosOperatorPredicate := predicate.NewPredicateFuncs(func(object client.Object) bool {
		labels := object.GetLabels()
		if labels == nil {
			return false
		}

		managedBy, exists := labels[manifests.ManagedByLabel]
		if !exists || managedBy != manifests.DefaultManagedByLabel {
			return false
		}

		partOf, exists := labels[manifests.PartOfLabel]
		if !exists || partOf != manifests.DefaultPartOfLabel {
			return false
		}

		return true
	})

	err := ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.StatefulSet{}).
		WithEventFilter(thanosOperatorPredicate).
		Complete(r)

	if err != nil {
		r.recorder.Eventf(&appsv1.StatefulSet{}, nil, corev1.EventTypeWarning, "SetupFailed", "Setup",
			"Failed to set up volume resize controller: %v", err)
		return fmt.Errorf("failed to set up volume resize controller: %w", err)
	}

	return nil
}
