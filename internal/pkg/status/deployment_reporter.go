// This file is heavily inspired by Prometheus Operator.
//
// Copyright 2022 The prometheus-operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package status

import (
	"context"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DeploymentReporter struct {
	Pods        []*Pod
	deployment  *appsv1.Deployment
	replicaSets *appsv1.ReplicaSetList
}

// NewDeploymentReporter returns a deployment's reporter.
func NewDeploymentReporter(ctx context.Context, c client.Client, deployment *appsv1.Deployment) (*DeploymentReporter, error) {
	if deployment == nil {
		// deployment is nil when the controller couldn't create the deployment
		// (incompatible spec fields for instance).
		return &DeploymentReporter{
			Pods: []*Pod{},
		}, nil
	}

	ls, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		// Something is really broken if the deployment's selector isn't valid.
		panic(err)
	}

	pods := &v1.PodList{}
	if err := c.List(ctx, pods, client.InNamespace(deployment.Namespace), client.MatchingLabelsSelector{Selector: ls}); err != nil {
		return nil, err
	}

	replicaSets := &appsv1.ReplicaSetList{}
	if err := c.List(ctx, replicaSets, client.InNamespace(deployment.Namespace), client.MatchingLabelsSelector{Selector: ls}); err != nil {
		return nil, err
	}

	deploymentReporter := &DeploymentReporter{
		deployment:  deployment,
		replicaSets: replicaSets,
		Pods:        make([]*Pod, 0, len(pods.Items)),
	}
	for _, p := range pods.Items {
		var found bool
		for _, owner := range p.ObjectMeta.OwnerReferences {
			if owner.Kind == "Deployment" && owner.Name == deployment.Name {
				found = true
				break
			}
		}

		if !found {
			continue
		}

		deploymentReporter.Pods = append(deploymentReporter.Pods, ptr.To(Pod(p)))
	}

	return deploymentReporter, nil
}

// UpdatedPods returns the list of pods that match with the deployment's revision.
func (dr *DeploymentReporter) UpdatedPods() []*Pod {
	return dr.filterPods(func(p *Pod) bool {
		return dr.IsUpdated(p)
	})
}

// IsUpdated returns true if the given pod matches with the deployment's revision.
func (dr *DeploymentReporter) IsUpdated(p *Pod) bool {
	for _, rs := range dr.replicaSets.Items {
		if rs.Labels["pod-template-hash"] == p.Labels["pod-template-hash"] {
			if dr.deployment.Annotations["deployment.kubernetes.io/revision"] == rs.Annotations["deployment.kubernetes.io/revision"] {
				return true
			}
		}
	}
	return false
}

// ReadyPods returns the list of pods that are ready.
func (dr *DeploymentReporter) ReadyPods() []*Pod {
	return dr.filterPods(func(p *Pod) bool {
		return p.Ready()
	})
}

func (dr *DeploymentReporter) filterPods(f func(*Pod) bool) []*Pod {
	pods := make([]*Pod, 0, len(dr.Pods))

	for _, p := range dr.Pods {
		if f(p) {
			pods = append(pods, p)
		}
	}

	return pods
}

func (dr *DeploymentReporter) Update(gObj GoverningObject) metav1.Condition {
	condition := metav1.Condition{
		Type:   "Available",
		Status: metav1.ConditionTrue,
		LastTransitionTime: metav1.Time{
			Time: time.Now().UTC(),
		},
		ObservedGeneration: gObj.GetGeneration(),
	}

	var (
		ready     = len(dr.ReadyPods())
		updated   = len(dr.UpdatedPods())
		available = len(dr.ReadyPods())
	)
	gObj.SetReplicas(len(dr.Pods))
	gObj.SetUpdatedReplicas(updated)
	gObj.SetAvailableReplicas(ready)
	gObj.SetUnavailableReplicas(len(dr.Pods) - ready)

	switch {
	case dr.deployment == nil:
		condition.Reason = "DeploymentNotFound"
		condition.Status = metav1.ConditionFalse
	case ready < gObj.ExpectedReplicas():
		switch {
		case available == 0:
			condition.Reason = "NoPodReady"
			condition.Status = metav1.ConditionFalse
		default:
			condition.Reason = "SomePodsNotReady"
			condition.Status = "Degraded"
		}
	}

	var messages []string
	for _, p := range dr.Pods {
		if m := p.Message(); m != "" {
			messages = append(messages, fmt.Sprintf("pod %s: %s", p.Name, m))
		}
	}

	condition.Message = strings.Join(messages, "\n")
	return condition
}
