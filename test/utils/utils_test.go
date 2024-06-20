package utils

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestVerifyStsReplicasRunning(t *testing.T) {
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

	ready := VerifyStsReplicasRunning(fake.NewClientBuilder().WithObjects(notReady).Build(), 3, name, ns)
	if ready {
		t.Errorf("expected not ready statefulset")
	}

	notReady.Status.ReadyReplicas = 3
	notReady.Status.AvailableReplicas = 3
	readySts := notReady.DeepCopy()
	ready = VerifyStsReplicasRunning(fake.NewClientBuilder().WithObjects(readySts).Build(), 3, name, ns)
	if !ready {
		t.Errorf("expected ready statefulset")
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
