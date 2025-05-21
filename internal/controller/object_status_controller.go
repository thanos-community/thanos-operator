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
	"time"

	"github.com/go-logr/logr"

	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/handlers"
	manifests "github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	compactbldr "github.com/thanos-community/thanos-operator/internal/pkg/manifests/compact"
	querybldr "github.com/thanos-community/thanos-operator/internal/pkg/manifests/query"
	queryfrontendbldr "github.com/thanos-community/thanos-operator/internal/pkg/manifests/queryfrontend"
	receivebldr "github.com/thanos-community/thanos-operator/internal/pkg/manifests/receive"
	rulerbldr "github.com/thanos-community/thanos-operator/internal/pkg/manifests/ruler"
	storebldr "github.com/thanos-community/thanos-operator/internal/pkg/manifests/store"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Define condition types and reasons
const (
	ConditionReconcileSuccess = "ReconcileSuccess"
	ConditionReconcileFailed  = "ReconcileFailed"
	ConditionPaused           = "Paused"

	ReasonReconcileComplete = "ReconcileComplete"
	ReasonReconcileError    = "ReconcileError"
	ReasonPaused            = "Paused"
)

// ObjectStatusReconciler reconciles status fields of ThanosOperator objects object
type ObjectStatusReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	logger logr.Logger
	// metrics  controllermetrics.ThanosQueryMetrics
	recorder record.EventRecorder

	handler *handlers.Handler
}

// NewObjectStatusReconciler returns a reconciler for ThanosQuery resources.
func NewObjectStatusReconciler(conf Config, client client.Client, scheme *runtime.Scheme) *ObjectStatusReconciler {
	handler := handlers.NewHandler(client, scheme, conf.InstrumentationConfig.Logger)
	featureGates := conf.FeatureGate.ToGVK()
	if len(featureGates) > 0 {
		handler.SetFeatureGates(featureGates)
	}

	return &ObjectStatusReconciler{
		Client: client,
		Scheme: scheme,
		logger: conf.InstrumentationConfig.Logger,
		// metrics:  controllermetrics.NewThanosQueryMetrics(conf.InstrumentationConfig.MetricsRegistry),
		recorder: conf.InstrumentationConfig.EventRecorder,
		handler:  handler,
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

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.0/pkg/reconcile
func (r *ObjectStatusReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.logger.Info("Reconciling ThanosQuery", "request", req)
	r.updateAllThanosQueryStatuses(ctx)
	r.updateAllThanosReceiveStatuses(ctx)
	r.updateAllThanosCompactStatuses(ctx)
	r.updateAllThanosRulerStatuses(ctx)
	r.updateAllThanosStoreStatuses(ctx)

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ObjectStatusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	err := ctrl.NewControllerManagedBy(mgr).
		Named("object_status_controller").
		Watches(&monitoringthanosiov1alpha1.ThanosQuery{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
			return []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      obj.GetName(),
						Namespace: obj.GetNamespace(),
					},
				},
			}
		})).
		Watches(&monitoringthanosiov1alpha1.ThanosCompact{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
			return []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      obj.GetName(),
						Namespace: obj.GetNamespace(),
					},
				},
			}
		})).
		Watches(&monitoringthanosiov1alpha1.ThanosReceive{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
			return []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      obj.GetName(),
						Namespace: obj.GetNamespace(),
					},
				},
			}
		})).
		Watches(&monitoringthanosiov1alpha1.ThanosStore{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
			return []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      obj.GetName(),
						Namespace: obj.GetNamespace(),
					},
				},
			}
		})).
		Watches(&monitoringthanosiov1alpha1.ThanosRuler{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
			return []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      obj.GetName(),
						Namespace: obj.GetNamespace(),
					},
				},
			}
		})).
		Complete(r)

	if err != nil {
		return err
	}

	return nil
}

// updateStatus updates the status of the resource.
func (r *ObjectStatusReconciler) updateStatus(ctx context.Context, object client.Object) {
	err := r.Status().Update(ctx, object)
	if err != nil {
		r.logger.Error(err, "failed to update status for object", "object", object.GetName())
	}
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
		client.MatchingLabels(object.GetLabels()),
	}
	err := r.List(ctx, &deploymentList, listOpts...)
	if err != nil {
		r.logger.Error(err, "failed to list deployments")
		return nil
	}

	var s []stats
	for _, deployment := range deploymentList.Items {
		isOwned := false
		for _, ownerRef := range deployment.OwnerReferences {
			if ownerRef.APIVersion == object.GetObjectKind().GroupVersionKind().GroupVersion().String() &&
				ownerRef.Kind == object.GetObjectKind().GroupVersionKind().Kind &&
				ownerRef.Name == object.GetName() {
				isOwned = true
				break
			}
		}
		if !isOwned {
			continue
		}

		containerNames := []string{}
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
		client.MatchingLabels(object.GetLabels()),
	}
	err := r.List(ctx, &statefulsetList, listOpts...)
	if err != nil {
		r.logger.Error(err, "failed to list statefulsets")
		return nil
	}

	var s []stats
	for _, statefulset := range statefulsetList.Items {
		isOwned := false
		for _, ownerRef := range statefulset.OwnerReferences {
			if ownerRef.APIVersion == object.GetObjectKind().GroupVersionKind().GroupVersion().String() &&
				ownerRef.Kind == object.GetObjectKind().GroupVersionKind().Kind &&
				ownerRef.Name == object.GetName() {
				isOwned = true
				break
			}
		}
		if !isOwned {
			continue
		}

		containerNames := []string{}
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

// updateAllThanosQueryStatuses updates the status of all ThanosQuery resources.
func (r *ObjectStatusReconciler) updateAllThanosQueryStatuses(ctx context.Context) {
	var queryList monitoringthanosiov1alpha1.ThanosQueryList
	err := r.List(ctx, &queryList)
	if err != nil {
		r.logger.Error(err, "failed to list ThanosQuery resources for status update")
		return
	}

	for _, query := range queryList.Items {
		deploymentStatuses := r.getDeploymentStatuses(ctx, &query)

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
		r.updateStatus(ctx, &query)
	}
}

// updateAllThanosReceiveStatuses updates the status of all ThanosReceive resources.
func (r *ObjectStatusReconciler) updateAllThanosReceiveStatuses(ctx context.Context) {
	var receiveList monitoringthanosiov1alpha1.ThanosReceiveList
	err := r.List(ctx, &receiveList)
	if err != nil {
		r.logger.Error(err, "failed to list ThanosReceive resources for status update")
		return
	}

	for _, receive := range receiveList.Items {
		deploymentStatuses := r.getDeploymentStatuses(ctx, &receive)
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

		receive.Status.HashringStatus = make(map[string]monitoringthanosiov1alpha1.IngesterStatus)

		statefulsetStatuses := r.getStatefulsetStatuses(ctx, &receive)
		for _, status := range statefulsetStatuses {
			for _, containerName := range status.containerNames {
				if containerName == receivebldr.IngestComponentName {
					hashringName := status.labels[manifests.HashringLabel]
					receive.Status.HashringStatus[hashringName] = monitoringthanosiov1alpha1.IngesterStatus{
						AvailableReplicas: status.availableReplicas,
						Replicas:          status.replicas,
						UpdatedReplicas:   status.updatedReplicas,
						ReadyReplicas:     status.readyReplicas,
						CurrentReplicas:   status.currentReplicas,
					}
				}
			}
		}

		r.updateStatus(ctx, &receive)
	}
}

// updateAllThanosCompactStatuses updates the status of all ThanosCompact resources.
func (r *ObjectStatusReconciler) updateAllThanosCompactStatuses(ctx context.Context) {
	var compactList monitoringthanosiov1alpha1.ThanosCompactList
	err := r.List(ctx, &compactList)
	if err != nil {
		r.logger.Error(err, "failed to list ThanosCompact resources for status update")
		return
	}

	for _, compact := range compactList.Items {
		statefulsetStatuses := r.getStatefulsetStatuses(ctx, &compact)
		compact.Status.ShardStatuses = make(map[string]monitoringthanosiov1alpha1.ShardStatus)
		for _, status := range statefulsetStatuses {
			for _, containerName := range status.containerNames {
				if containerName == compactbldr.Name {
					r.logger.Info("Updating ThanosCompact statuses", "containerName", containerName, "status", status)
					shardName, ok := status.labels[manifests.ShardLabel]
					if !ok {
						compact.Status.ShardStatuses["default"] = monitoringthanosiov1alpha1.ShardStatus{
							AvailableReplicas: status.availableReplicas,
							Replicas:          status.replicas,
							UpdatedReplicas:   status.updatedReplicas,
							ReadyReplicas:     status.readyReplicas,
							CurrentReplicas:   status.currentReplicas,
						}
					} else {
						compact.Status.ShardStatuses[shardName] = monitoringthanosiov1alpha1.ShardStatus{
							AvailableReplicas: status.availableReplicas,
							Replicas:          status.replicas,
							UpdatedReplicas:   status.updatedReplicas,
							ReadyReplicas:     status.readyReplicas,
							CurrentReplicas:   status.currentReplicas,
						}
					}
				}
			}
		}

		r.updateStatus(ctx, &compact)
	}
}

// updateAllThanosRulerStatuses updates the status of all ThanosRuler resources.
func (r *ObjectStatusReconciler) updateAllThanosRulerStatuses(ctx context.Context) {
	var rulerList monitoringthanosiov1alpha1.ThanosRulerList
	err := r.List(ctx, &rulerList)
	if err != nil {
		r.logger.Error(err, "failed to list ThanosRuler resources for status update")
		return
	}

	for _, ruler := range rulerList.Items {
		statefulsetStatuses := r.getStatefulsetStatuses(ctx, &ruler)
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
		r.updateStatus(ctx, &ruler)
	}
}

// updateAllThanosStoreStatuses updates the status of all ThanosStore resources.
func (r *ObjectStatusReconciler) updateAllThanosStoreStatuses(ctx context.Context) {
	var storeList monitoringthanosiov1alpha1.ThanosStoreList
	err := r.List(ctx, &storeList)
	if err != nil {
		r.logger.Error(err, "failed to list ThanosStore resources for status update")
		return
	}

	for _, store := range storeList.Items {
		statefulsetStatuses := r.getStatefulsetStatuses(ctx, &store)
		store.Status.ShardStatuses = make(map[string]monitoringthanosiov1alpha1.ShardStatus)
		for _, status := range statefulsetStatuses {
			for _, containerName := range status.containerNames {
				if containerName == storebldr.Name {
					shardName, ok := status.labels[manifests.ShardLabel]
					if !ok {
						store.Status.ShardStatuses["default"] = monitoringthanosiov1alpha1.ShardStatus{
							AvailableReplicas: status.availableReplicas,
							Replicas:          status.replicas,
							UpdatedReplicas:   status.updatedReplicas,
							ReadyReplicas:     status.readyReplicas,
							CurrentReplicas:   status.currentReplicas,
						}
					} else {
						store.Status.ShardStatuses[shardName] = monitoringthanosiov1alpha1.ShardStatus{
							AvailableReplicas: status.availableReplicas,
							Replicas:          status.replicas,
							UpdatedReplicas:   status.updatedReplicas,
							ReadyReplicas:     status.readyReplicas,
							CurrentReplicas:   status.currentReplicas,
						}
					}
				}
			}
		}

		r.updateStatus(ctx, &store)
	}
}
