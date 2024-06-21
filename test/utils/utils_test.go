package utils

import (
	"net/http"
	"testing"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestVerifyServiceExists(t *testing.T) {
	const (
		name = "test"
		ns   = "test"
	)

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
	}

	ok := VerifyServiceExists(fake.NewClientBuilder().WithObjects(svc).Build(), name, ns)
	if !ok {
		t.Errorf("expected service")
	}

	ok = VerifyServiceExists(fake.NewClientBuilder().Build(), name, ns)
	if ok {
		t.Errorf("unexpected service")
	}
}

func TestVerifyServiceAccountExists(t *testing.T) {
	const (
		name = "test"
		ns   = "test"
	)

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
	}

	ok := VerifyServiceAccountExists(fake.NewClientBuilder().WithObjects(sa).Build(), name, ns)
	if !ok {
		t.Errorf("expected serviceaccount")
	}

	ok = VerifyServiceAccountExists(fake.NewClientBuilder().Build(), name, ns)
	if ok {
		t.Errorf("unexpected serviceaccount")
	}
}

func TestVerifyDeploymentExists(t *testing.T) {
	const (
		name = "test"
		ns   = "test"
	)

	d := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
	}

	ok := VerifyDeploymentExists(fake.NewClientBuilder().WithObjects(d).Build(), name, ns)
	if !ok {
		t.Errorf("expected deployment")
	}

	ok = VerifyDeploymentExists(fake.NewClientBuilder().Build(), name, ns)
	if ok {
		t.Errorf("unexpected deployment")
	}
}

func TestVerifyStatefulSetExists(t *testing.T) {
	const (
		name = "test"
		ns   = "test"
	)

	sts := &v1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
	}

	ok := VerifyStatefulSetExists(fake.NewClientBuilder().WithObjects(sts).Build(), name, ns)
	if !ok {
		t.Errorf("expected statefulset")
	}

	ok = VerifyStatefulSetExists(fake.NewClientBuilder().Build(), name, ns)
	if ok {
		t.Errorf("unexpected statefulset")
	}
}

func TestVerifyStatefulSetReplicasRunning(t *testing.T) {
	const (
		name = "test"
		ns   = "test"
	)

	notReady := &v1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: v1.StatefulSetSpec{
			Replicas: ptr.To(int32(3)),
		},
		Status: v1.StatefulSetStatus{
			Replicas:          3,
			ReadyReplicas:     2,
			AvailableReplicas: 2,
		},
	}

	fake.NewClientBuilder().WithObjects(notReady).Build()

	ready := VerifyStatefulSetReplicasRunning(fake.NewClientBuilder().WithObjects(notReady).Build(), 3, name, ns)
	if ready {
		t.Errorf("expected not ready statefulset")
	}

	notReady.Status.ReadyReplicas = 3
	notReady.Status.AvailableReplicas = 3
	readySts := notReady.DeepCopy()
	ready = VerifyStatefulSetReplicasRunning(fake.NewClientBuilder().WithObjects(readySts).Build(), 3, name, ns)
	if !ready {
		t.Errorf("expected ready statefulset")
	}
}

func TestVerifyDeploymentReplicasRunning(t *testing.T) {
	const (
		name = "test"
		ns   = "test"
	)

	notReady := &v1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: v1.DeploymentSpec{
			Replicas: ptr.To(int32(3)),
		},
		Status: v1.DeploymentStatus{
			Replicas:      3,
			ReadyReplicas: 2,
		},
	}

	fake.NewClientBuilder().WithObjects(notReady).Build()

	ready := VerifyDeploymentReplicasRunning(fake.NewClientBuilder().WithObjects(notReady).Build(), 3, name, ns)
	if ready {
		t.Errorf("expected not ready deployment")
	}

	notReady.Status.ReadyReplicas = 3
	readyDeployment := notReady.DeepCopy()
	ready = VerifyDeploymentReplicasRunning(fake.NewClientBuilder().WithObjects(readyDeployment).Build(), 3, name, ns)
	if !ready {
		t.Errorf("expected ready deployment")
	}
}

func TestVerifyConfigMapContents(t *testing.T) {
	const (
		name = "test"
		ns   = "test"
	)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Data: map[string]string{
			"key": "value",
		},
	}

	ready := VerifyConfigMapContents(fake.NewClientBuilder().WithObjects(cm).Build(), name, ns, "key", "value")
	if !ready {
		t.Errorf("expected ready configmap")
	}

	ready = VerifyConfigMapContents(fake.NewClientBuilder().WithObjects(cm).Build(), name, ns, "key", "notvalue")
	if ready {
		t.Errorf("expected not ready configmap")
	}
}

func TestVerifyDeploymentArgs(t *testing.T) {
	const (
		name = "test"
		ns   = "test"
	)

	d := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: v1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: "test",
							Args:  []string{"arg1", "arg2"},
						},
					},
				},
			},
		},
	}

	ok := VerifyDeploymentArgs(fake.NewClientBuilder().WithObjects(d).Build(), name, ns, "arg1")
	if !ok {
		t.Errorf("expected arg")
	}

	ok = VerifyDeploymentArgs(fake.NewClientBuilder().WithObjects(d).Build(), name, ns, "na")
	if ok {
		t.Errorf("unexpected arg")
	}
}

func TestVerifyExistenceOfRequiredNamedResources(t *testing.T) {
	const (
		name = "test"
		ns   = "test"
	)

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
	}

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
	}

	deployment := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
	}

	sts := &v1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
	}

	ok := VerifyExistenceOfRequiredNamedResources(
		fake.NewClientBuilder().WithObjects(svc, sa, cm, deployment, sts).Build(),
		ExpectApiResourceDeployment,
		name, ns)
	if !ok {
		t.Errorf("expected resources")
	}

	ok = VerifyExistenceOfRequiredNamedResources(
		fake.NewClientBuilder().WithObjects(svc, sa, cm, deployment).Build(),
		ExpectApiResourceStatefulSet,
		name, ns)
	if ok {
		t.Errorf("unexpected resources")
	}

}

func TestRemoteWrite(t *testing.T) {
	err := RemoteWrite(DefaultRemoteWriteRequest(), roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadGateway,
		}, nil
	}), nil)
	if err == nil {
		t.Errorf("expected error")
	}

	err = RemoteWrite(DefaultRemoteWriteRequest(), roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
		}, nil
	}), nil)
	if err != nil {
		t.Errorf("expected no error")
	}
}

type roundTripFunc func(r *http.Request) (*http.Response, error)

func (s roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return s(r)
}
