package status

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// GoverningObject is an interface that represents a Kubernetes object that can be used to report the status of a StatefulSet or Deployment.
type GoverningObject interface {
	metav1.Object
	ExpectedReplicas() int
	SetReplicas(int)
	SetUpdatedReplicas(int)
	SetAvailableReplicas(int)
	SetUnavailableReplicas(int)
}

type Reporter interface {
	Update(GoverningObject) metav1.Condition
}

var _ Reporter = &StatefulSetReporter{}
var _ Reporter = &DeploymentReporter{}
