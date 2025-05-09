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

type StatefulSetReporter struct {
	Pods []*Pod
	sset *appsv1.StatefulSet
}

// NewStatefulSetReporter returns a statefulset's reporter.
func NewStatefulSetReporter(ctx context.Context, c client.Client, sset *appsv1.StatefulSet) (*StatefulSetReporter, error) {
	if sset == nil {
		// sset is nil when the controller couldn't create the statefulset
		// (incompatible spec fields for instance).
		return &StatefulSetReporter{
			Pods: []*Pod{},
		}, nil
	}

	ls, err := metav1.LabelSelectorAsSelector(sset.Spec.Selector)
	if err != nil {
		// Something is really broken if the statefulset's selector isn't valid.
		panic(err)
	}

	pods := &v1.PodList{}
	if err := c.List(ctx, pods, client.InNamespace(sset.Namespace), client.MatchingLabelsSelector{Selector: ls}); err != nil {
		return nil, err
	}

	stsReporter := &StatefulSetReporter{
		sset: sset,
		Pods: make([]*Pod, 0, len(pods.Items)),
	}
	for _, p := range pods.Items {
		var found bool
		for _, owner := range p.ObjectMeta.OwnerReferences {
			if owner.Kind == "StatefulSet" && owner.Name == sset.Name {
				found = true
				break
			}
		}

		if !found {
			continue
		}

		stsReporter.Pods = append(stsReporter.Pods, ptr.To(Pod(p)))
	}

	return stsReporter, nil
}

// UpdatedPods returns the list of pods that match with the statefulset's revision.
func (sr *StatefulSetReporter) UpdatedPods() []*Pod {
	return sr.filterPods(func(p *Pod) bool {
		return sr.IsUpdated(p)
	})
}

// IsUpdated returns true if the given pod matches with the statefulset's revision.
func (sr *StatefulSetReporter) IsUpdated(p *Pod) bool {
	return sr.sset.Status.UpdateRevision == p.Labels["controller-revision-hash"]
}

// ReadyPods returns the list of pods that are ready.
func (sr *StatefulSetReporter) ReadyPods() []*Pod {
	return sr.filterPods(func(p *Pod) bool {
		return p.Ready()
	})
}

func (sr *StatefulSetReporter) filterPods(f func(*Pod) bool) []*Pod {
	pods := make([]*Pod, 0, len(sr.Pods))

	for _, p := range sr.Pods {
		if f(p) {
			pods = append(pods, p)
		}
	}

	return pods
}

func (sr *StatefulSetReporter) Update(gObj GoverningObject) metav1.Condition {
	condition := metav1.Condition{
		Type:   "Available",
		Status: metav1.ConditionTrue,
		LastTransitionTime: metav1.Time{
			Time: time.Now().UTC(),
		},
		ObservedGeneration: gObj.GetGeneration(),
	}

	var (
		ready     = len(sr.ReadyPods())
		updated   = len(sr.UpdatedPods())
		available = len(sr.ReadyPods())
	)
	gObj.SetReplicas(len(sr.Pods))
	gObj.SetUpdatedReplicas(updated)
	gObj.SetAvailableReplicas(ready)
	gObj.SetUnavailableReplicas(len(sr.Pods) - ready)

	switch {
	case sr.sset == nil:
		condition.Reason = "StatefulSetNotFound"
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
	for _, p := range sr.Pods {
		if m := p.Message(); m != "" {
			messages = append(messages, fmt.Sprintf("pod %s: %s", p.Name, m))
		}
	}

	condition.Message = strings.Join(messages, "\n")

	return condition
}
