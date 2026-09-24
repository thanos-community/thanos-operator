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
	"reflect"

	"github.com/go-logr/logr"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"

	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/handlers"
	manifests "github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	compactbldr "github.com/thanos-community/thanos-operator/internal/pkg/manifests/compact"
	querybldr "github.com/thanos-community/thanos-operator/internal/pkg/manifests/query"
	queryfrontendbldr "github.com/thanos-community/thanos-operator/internal/pkg/manifests/queryfrontend"
	receivebldr "github.com/thanos-community/thanos-operator/internal/pkg/manifests/receive"
	rulerbldr "github.com/thanos-community/thanos-operator/internal/pkg/manifests/ruler"
	storebldr "github.com/thanos-community/thanos-operator/internal/pkg/manifests/store"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Define condition types and reasons
const (
	ConditionReady  = "Ready"
	ConditionPaused = "Paused"

	ReasonReconcileComplete = "ReconcileComplete"
	ReasonReconcileError    = "ReconcileError"
	ReasonPaused            = "Paused"
)

var thanosOwnerKinds = map[string]struct{}{
	"ThanosQuery":   {},
	"ThanosReceive": {},
	"ThanosCompact": {},
	"ThanosStore":   {},
	"ThanosRuler":   {},
}

// ObjectStatusReconciler reconciles replica status fields of Thanos operator CRs.
type ObjectStatusReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	logger logr.Logger
	// metrics  controllermetrics.ThanosQueryMetrics
	recorder events.EventRecorder

	handler *handlers.Handler
}

// NewObjectStatusReconciler returns a reconciler for Thanos CR replica status.
func NewObjectStatusReconciler(conf Config, client client.Client, scheme *runtime.Scheme) *ObjectStatusReconciler {
	return &ObjectStatusReconciler{
		Client:   client,
		Scheme:   scheme,
		logger:   conf.InstrumentationConfig.Logger,
		recorder: conf.InstrumentationConfig.EventRecorder,
		handler:  handlers.NewHandler(client, scheme, conf.InstrumentationConfig.Logger),
	}
}

//+kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosqueries,verbs=get;list;watch
//+kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosqueries/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosreceives,verbs=get;list;watch
//+kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosreceives/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosstores,verbs=get;list;watch
//+kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosstores/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanoscompacts,verbs=get;list;watch
//+kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanoscompacts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosrulers,verbs=get;list;watch
//+kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosrulers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps,resources=statefulsets;deployments,verbs=get;list;watch

// Reconcile updates replica status for the Thanos CR identified by req.
func (r *ObjectStatusReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if handled, err := r.reconcileThanosQueryStatus(ctx, req); err != nil {
		return ctrl.Result{}, err
	} else if handled {
		return ctrl.Result{}, nil
	}

	if handled, err := r.reconcileThanosReceiveStatus(ctx, req); err != nil {
		return ctrl.Result{}, err
	} else if handled {
		return ctrl.Result{}, nil
	}

	if handled, err := r.reconcileThanosCompactStatus(ctx, req); err != nil {
		return ctrl.Result{}, err
	} else if handled {
		return ctrl.Result{}, nil
	}

	if handled, err := r.reconcileThanosRulerStatus(ctx, req); err != nil {
		return ctrl.Result{}, err
	} else if handled {
		return ctrl.Result{}, nil
	}

	if handled, err := r.reconcileThanosStoreStatus(ctx, req); err != nil {
		return ctrl.Result{}, err
	} else if handled {
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ObjectStatusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	enqueueSelf := handler.EnqueueRequestsFromMapFunc(func(_ context.Context, obj client.Object) []reconcile.Request {
		return []reconcile.Request{{NamespacedName: types.NamespacedName{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
		}}}
	})

	return ctrl.NewControllerManagedBy(mgr).
		Named("object_status_controller").
		Watches(&monitoringthanosiov1alpha1.ThanosQuery{}, enqueueSelf).
		Watches(&monitoringthanosiov1alpha1.ThanosCompact{}, enqueueSelf).
		Watches(&monitoringthanosiov1alpha1.ThanosReceive{}, enqueueSelf).
		Watches(&monitoringthanosiov1alpha1.ThanosStore{}, enqueueSelf).
		Watches(&monitoringthanosiov1alpha1.ThanosRuler{}, enqueueSelf).
		Watches(
			&appsv1.Deployment{},
			r.enqueueForWorkload(),
			builder.WithPredicates(predicate.NewPredicateFuncs(isThanosManagedWorkload)),
		).
		Watches(
			&appsv1.StatefulSet{},
			r.enqueueForWorkload(),
			builder.WithPredicates(predicate.NewPredicateFuncs(isThanosManagedWorkload)),
		).
		Complete(r)
}

func isThanosManagedWorkload(obj client.Object) bool {
	labels := obj.GetLabels()
	if labels == nil {
		return false
	}
	return labels[manifests.ManagedByLabel] == manifests.DefaultManagedByLabel &&
		labels[manifests.PartOfLabel] == manifests.DefaultPartOfLabel
}

func (r *ObjectStatusReconciler) enqueueForWorkload() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(_ context.Context, obj client.Object) []reconcile.Request {
		for _, ownerRef := range obj.GetOwnerReferences() {
			if _, ok := thanosOwnerKinds[ownerRef.Kind]; !ok {
				continue
			}
			return []reconcile.Request{{
				NamespacedName: types.NamespacedName{
					Name:      ownerRef.Name,
					Namespace: obj.GetNamespace(),
				},
			}}
		}
		return nil
	})
}

func (r *ObjectStatusReconciler) patchReplicaStatus(ctx context.Context, key types.NamespacedName, obj client.Object, apply func(client.Object) bool) error {
	latest := obj.DeepCopyObject().(client.Object)
	if err := r.Get(ctx, key, latest); err != nil {
		return err
	}

	if !apply(latest) {
		return nil
	}

	// Use Update rather than Patch: DeploymentStatus CRD fields are required when
	// querierStatus/routerStatus is set, and merge patches can omit zero values.
	return r.Status().Update(ctx, latest)
}

type stats struct {
	name                string
	labels              map[string]string
	containerNames      []string
	availableReplicas   int32
	replicas            int32
	readyReplicas       int32
	updatedReplicas     int32
	unavailableReplicas int32
	currentReplicas     int32
}

func (r *ObjectStatusReconciler) getDeploymentStatuses(ctx context.Context, object client.Object) []stats {
	var deploymentList appsv1.DeploymentList
	listOpts := []client.ListOption{
		client.InNamespace(object.GetNamespace()),
		client.MatchingLabels{manifests.OwnerLabel: object.GetName()},
	}
	if err := r.List(ctx, &deploymentList, listOpts...); err != nil {
		r.logger.Error(err, "failed to list deployments")
		return nil
	}

	gvk := object.GetObjectKind().GroupVersionKind()
	s := make([]stats, 0)
	for _, deployment := range deploymentList.Items {
		if !isOwnedBy(deployment.OwnerReferences, gvk, object.GetName()) {
			continue
		}

		containerNames := make([]string, 0, len(deployment.Spec.Template.Spec.Containers))
		for _, container := range deployment.Spec.Template.Spec.Containers {
			containerNames = append(containerNames, container.Name)
		}

		s = append(s, stats{
			name:                deployment.Name,
			labels:              deployment.Labels,
			containerNames:      containerNames,
			availableReplicas:   deployment.Status.AvailableReplicas,
			replicas:            deployment.Status.Replicas,
			updatedReplicas:     deployment.Status.UpdatedReplicas,
			unavailableReplicas: deployment.Status.UnavailableReplicas,
			readyReplicas:       deployment.Status.ReadyReplicas,
		})
	}

	return s
}

func (r *ObjectStatusReconciler) getStatefulsetStatuses(ctx context.Context, object client.Object) []stats {
	var statefulsetList appsv1.StatefulSetList
	listOpts := []client.ListOption{
		client.InNamespace(object.GetNamespace()),
		client.MatchingLabels{manifests.OwnerLabel: object.GetName()},
	}
	if err := r.List(ctx, &statefulsetList, listOpts...); err != nil {
		r.logger.Error(err, "failed to list statefulsets")
		return nil
	}

	gvk := object.GetObjectKind().GroupVersionKind()
	s := make([]stats, 0)
	for _, statefulset := range statefulsetList.Items {
		if !isOwnedBy(statefulset.OwnerReferences, gvk, object.GetName()) {
			continue
		}

		containerNames := make([]string, 0, len(statefulset.Spec.Template.Spec.Containers))
		for _, container := range statefulset.Spec.Template.Spec.Containers {
			containerNames = append(containerNames, container.Name)
		}

		s = append(s, stats{
			name:              statefulset.Name,
			labels:            statefulset.Labels,
			containerNames:    containerNames,
			availableReplicas: statefulset.Status.AvailableReplicas,
			replicas:          statefulset.Status.Replicas,
			updatedReplicas:   statefulset.Status.UpdatedReplicas,
			readyReplicas:     statefulset.Status.ReadyReplicas,
			currentReplicas:   statefulset.Status.CurrentReplicas,
		})
	}

	return s
}

func isOwnedBy(ownerRefs []metav1.OwnerReference, gvk schema.GroupVersionKind, name string) bool {
	for _, ownerRef := range ownerRefs {
		if ownerRef.APIVersion == gvk.GroupVersion().String() &&
			ownerRef.Kind == gvk.Kind &&
			ownerRef.Name == name {
			return true
		}
	}
	return false
}

func (r *ObjectStatusReconciler) reconcileThanosQueryStatus(ctx context.Context, req ctrl.Request) (bool, error) {
	query := &monitoringthanosiov1alpha1.ThanosQuery{}
	if err := r.Get(ctx, req.NamespacedName, query); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	deploymentStatuses := r.getDeploymentStatuses(ctx, query)
	err := r.patchReplicaStatus(ctx, req.NamespacedName, query, func(obj client.Object) bool {
		query := obj.(*monitoringthanosiov1alpha1.ThanosQuery)
		original := query.Status.DeepCopy()

		for _, status := range deploymentStatuses {
			for _, containerName := range status.containerNames {
				if containerName == querybldr.Name {
					query.Status.Querier.AvailableReplicas = status.availableReplicas
					query.Status.Querier.Replicas = status.replicas
					query.Status.Querier.UpdatedReplicas = status.updatedReplicas
					query.Status.Querier.UnavailableReplicas = status.unavailableReplicas
					query.Status.Querier.ReadyReplicas = status.readyReplicas
				}
				if containerName == queryfrontendbldr.Name {
					query.Status.QueryFrontend.AvailableReplicas = status.availableReplicas
					query.Status.QueryFrontend.Replicas = status.replicas
					query.Status.QueryFrontend.UpdatedReplicas = status.updatedReplicas
					query.Status.QueryFrontend.UnavailableReplicas = status.unavailableReplicas
					query.Status.QueryFrontend.ReadyReplicas = status.readyReplicas
				}
			}
		}

		return !reflect.DeepEqual(original, &query.Status)
	})
	if err != nil {
		r.logger.Error(err, "failed to patch ThanosQuery replica status", "name", query.Name)
		return true, err
	}

	return true, nil
}

func (r *ObjectStatusReconciler) reconcileThanosReceiveStatus(ctx context.Context, req ctrl.Request) (bool, error) {
	receive := &monitoringthanosiov1alpha1.ThanosReceive{}
	if err := r.Get(ctx, req.NamespacedName, receive); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	deploymentStatuses := r.getDeploymentStatuses(ctx, receive)
	statefulsetStatuses := r.getStatefulsetStatuses(ctx, receive)
	err := r.patchReplicaStatus(ctx, req.NamespacedName, receive, func(obj client.Object) bool {
		receive := obj.(*monitoringthanosiov1alpha1.ThanosReceive)
		original := receive.Status.DeepCopy()

		for _, status := range deploymentStatuses {
			for _, containerName := range status.containerNames {
				if containerName == receivebldr.RouterComponentName {
					receive.Status.Router.AvailableReplicas = status.availableReplicas
					receive.Status.Router.Replicas = status.replicas
					receive.Status.Router.UpdatedReplicas = status.updatedReplicas
					receive.Status.Router.UnavailableReplicas = status.unavailableReplicas
					receive.Status.Router.ReadyReplicas = status.readyReplicas
				}
			}
		}

		receive.Status.HashringStatus = make(map[string]monitoringthanosiov1alpha1.StatefulSetStatus)
		for _, status := range statefulsetStatuses {
			for _, containerName := range status.containerNames {
				if containerName == receivebldr.IngestComponentName {
					hashringName := status.labels[manifests.HashringLabel]
					if hashringName == "" {
						hashringName = "default"
					}
					receive.Status.HashringStatus[hashringName] = monitoringthanosiov1alpha1.StatefulSetStatus{
						AvailableReplicas: status.availableReplicas,
						Replicas:          status.replicas,
						UpdatedReplicas:   status.updatedReplicas,
						ReadyReplicas:     status.readyReplicas,
						CurrentReplicas:   status.currentReplicas,
					}
				}
			}
		}

		return !reflect.DeepEqual(original, &receive.Status)
	})
	if err != nil {
		r.logger.Error(err, "failed to patch ThanosReceive replica status", "name", receive.Name)
		return true, err
	}

	return true, nil
}

func (r *ObjectStatusReconciler) reconcileThanosCompactStatus(ctx context.Context, req ctrl.Request) (bool, error) {
	compact := &monitoringthanosiov1alpha1.ThanosCompact{}
	if err := r.Get(ctx, req.NamespacedName, compact); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	statefulsetStatuses := r.getStatefulsetStatuses(ctx, compact)
	err := r.patchReplicaStatus(ctx, req.NamespacedName, compact, func(obj client.Object) bool {
		compact := obj.(*monitoringthanosiov1alpha1.ThanosCompact)
		original := compact.Status.DeepCopy()

		compact.Status.ShardStatuses = make(map[string]monitoringthanosiov1alpha1.StatefulSetStatus)
		for _, status := range statefulsetStatuses {
			for _, containerName := range status.containerNames {
				if containerName != compactbldr.Name {
					continue
				}

				shardName, ok := status.labels[manifests.ShardLabel]
				if !ok {
					shardName = "default"
				}
				compact.Status.ShardStatuses[shardName] = monitoringthanosiov1alpha1.StatefulSetStatus{
					AvailableReplicas: status.availableReplicas,
					Replicas:          status.replicas,
					UpdatedReplicas:   status.updatedReplicas,
					ReadyReplicas:     status.readyReplicas,
					CurrentReplicas:   status.currentReplicas,
				}
			}
		}

		return !reflect.DeepEqual(original, &compact.Status)
	})
	if err != nil {
		r.logger.Error(err, "failed to patch ThanosCompact replica status", "name", compact.Name)
		return true, err
	}

	return true, nil
}

func (r *ObjectStatusReconciler) reconcileThanosRulerStatus(ctx context.Context, req ctrl.Request) (bool, error) {
	ruler := &monitoringthanosiov1alpha1.ThanosRuler{}
	if err := r.Get(ctx, req.NamespacedName, ruler); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	statefulsetStatuses := r.getStatefulsetStatuses(ctx, ruler)
	err := r.patchReplicaStatus(ctx, req.NamespacedName, ruler, func(obj client.Object) bool {
		ruler := obj.(*monitoringthanosiov1alpha1.ThanosRuler)
		original := ruler.Status.DeepCopy()

		for _, status := range statefulsetStatuses {
			for _, containerName := range status.containerNames {
				if containerName == rulerbldr.Name {
					ruler.Status.AvailableReplicas = status.availableReplicas
					ruler.Status.Replicas = status.replicas
					ruler.Status.UpdatedReplicas = status.updatedReplicas
					ruler.Status.ReadyReplicas = status.readyReplicas
					ruler.Status.CurrentReplicas = status.currentReplicas
				}
			}
		}

		return !reflect.DeepEqual(original, &ruler.Status)
	})
	if err != nil {
		r.logger.Error(err, "failed to patch ThanosRuler replica status", "name", ruler.Name)
		return true, err
	}

	return true, nil
}

func (r *ObjectStatusReconciler) reconcileThanosStoreStatus(ctx context.Context, req ctrl.Request) (bool, error) {
	store := &monitoringthanosiov1alpha1.ThanosStore{}
	if err := r.Get(ctx, req.NamespacedName, store); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	statefulsetStatuses := r.getStatefulsetStatuses(ctx, store)
	err := r.patchReplicaStatus(ctx, req.NamespacedName, store, func(obj client.Object) bool {
		store := obj.(*monitoringthanosiov1alpha1.ThanosStore)
		original := store.Status.DeepCopy()

		store.Status.ShardStatuses = make(map[string]monitoringthanosiov1alpha1.StatefulSetStatus)
		for _, status := range statefulsetStatuses {
			for _, containerName := range status.containerNames {
				if containerName != storebldr.Name {
					continue
				}

				shardName, ok := status.labels[manifests.ShardLabel]
				if !ok {
					shardName = "default"
				}
				store.Status.ShardStatuses[shardName] = monitoringthanosiov1alpha1.StatefulSetStatus{
					AvailableReplicas: status.availableReplicas,
					Replicas:          status.replicas,
					UpdatedReplicas:   status.updatedReplicas,
					ReadyReplicas:     status.readyReplicas,
					CurrentReplicas:   status.currentReplicas,
				}
			}
		}

		return !reflect.DeepEqual(original, &store.Status)
	})
	if err != nil {
		r.logger.Error(err, "failed to patch ThanosStore replica status", "name", store.Name)
		return true, err
	}

	return true, nil
}
