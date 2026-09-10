package controller

import (
	"context"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// deleteOwnedServiceMonitors deletes all monitors controlled by owner.
func deleteOwnedServiceMonitors(ctx context.Context, c client.Client, owner client.Object) int {
	if owner.GetUID() == "" {
		return 0
	}

	var monitors monitoringv1.ServiceMonitorList
	if err := c.List(ctx, &monitors, client.InNamespace(owner.GetNamespace())); err != nil {
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			return 0
		}
		log.FromContext(ctx).Error(err, "failed to list ServiceMonitors")
		return 1
	}

	var errCount int
	for i := range monitors.Items {
		monitor := &monitors.Items[i]
		if !metav1.IsControlledBy(monitor, owner) {
			continue
		}
		if err := c.Delete(ctx, monitor); err != nil && !apierrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
			log.FromContext(ctx).Error(err, "failed to delete ServiceMonitor", "name", monitor.Name)
			errCount++
		}
	}
	return errCount
}
