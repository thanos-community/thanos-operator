package status

import v1 "k8s.io/api/core/v1"

// Pod is an alias for the Kubernetes v1.Pod type.
type Pod v1.Pod

// Ready returns true if the pod is ready.
func (p *Pod) Ready() bool {
	if p.Status.Phase != v1.PodRunning {
		return false
	}

	for _, cond := range p.Status.Conditions {
		if cond.Type != v1.PodReady {
			continue
		}
		return cond.Status == v1.ConditionTrue
	}

	return false
}

// Message returns a human-readable and terse message about the state of the pod.
func (p *Pod) Message() string {
	for _, condType := range []v1.PodConditionType{
		v1.PodScheduled,    // Check first that the pod is scheduled.
		v1.PodInitialized,  // Then that init containers have been started successfully.
		v1.ContainersReady, // Then that all containers are ready.
		v1.PodReady,        // And finally that the pod is ready.
	} {
		for _, cond := range p.Status.Conditions {
			if cond.Type == condType && cond.Status == v1.ConditionFalse {
				return cond.Message
			}
		}
	}

	return ""
}
