package k8s

import (
	"context"
	"reflect"
	"testing"

	"github.com/thanos-community/thanos-operator/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetDependentAnnotations(t *testing.T) {
	someSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "test-ns",
			Annotations: map[string]string{
				corev1.ServiceAccountNameKey: "test-sa",
			},
		},
	}

	someServiceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "test-ns",
			Annotations: map[string]string{
				"test": "test",
			},
			UID: types.UID("test-uid"),
		},
	}

	client := fake.NewClientBuilder().WithObjects(someServiceAccount, someSecret).Build()

	annotations, err := GetDependentAnnotations(context.Background(), client, someServiceAccount)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(annotations, map[string]string(nil)) {
		t.Fatal("unexpected annotations")
	}

	annotations, err = GetDependentAnnotations(context.Background(), client, someSecret)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(annotations, map[string]string{corev1.ServiceAccountUIDKey: "test-uid"}) {
		t.Fatal("unexpected annotations")
	}
}

func TestToSecretKeySelector(t *testing.T) {
	from := v1alpha1.ObjectStorageConfig{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: "test",
		},
		Key: "test",
	}

	expect := corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: "test",
		},
		Key:      "test",
		Optional: ptr.To(false),
	}
	result := ToSecretKeySelector(from)
	if !reflect.DeepEqual(result, expect) {
		t.Fatalf("unexpected result: %v", result)
	}
}
