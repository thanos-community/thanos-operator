package handlers

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type fakeClientWithError struct {
	client.Client
	shouldError bool
}

func (fc *fakeClientWithError) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if fc.shouldError {
		return fmt.Errorf("error")
	}
	return fc.Client.Update(ctx, obj)
}

func TestHandler_CreateOrUpdate(t *testing.T) {
	ctx := context.Background()
	const (
		namespace = "test"
		name      = "test"
	)

	owner := &appsv1.StatefulSet{}
	runTimeSts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	baseObjects := []client.Object{
		runTimeSts,
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		},
	}

	for _, tc := range []struct {
		name           string
		h              func() *Handler
		objs           []client.Object
		expectErrCount int
	}{
		{
			name: "test no errors on create fresh objects",
			h: func() *Handler {
				return &Handler{
					client: fake.NewFakeClient(),
					scheme: scheme.Scheme,
					logger: logr.New(log.NullLogSink{}),
				}
			},
			objs: baseObjects,
		},
		{
			name: "test error on update returns correct error count",
			h: func() *Handler {
				return &Handler{
					client: &fakeClientWithError{
						Client:      fake.NewFakeClient(runTimeSts),
						shouldError: true,
					},
					scheme: scheme.Scheme,
					logger: logr.New(log.NullLogSink{}),
				}
			},
			objs:           baseObjects,
			expectErrCount: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := tc.h()
			errCount := h.CreateOrUpdate(ctx, namespace, owner, tc.objs)
			if errCount != tc.expectErrCount {
				t.Errorf("expected %d errors, got %d", tc.expectErrCount, errCount)
			}
		})
	}

}
