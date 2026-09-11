package controller

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/thanos-community/thanos-operator/internal/pkg/handlers"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/query"
)

func TestTLSFanoutUsesReadyEndpointSliceTargets(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, discoveryv1.AddToScheme(scheme))
	slice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{Name: "store-a", Namespace: "metrics", Labels: map[string]string{discoveryv1.LabelServiceName: "store"}},
		Ports:      []discoveryv1.EndpointPort{{Name: ptr.To("grpc"), Port: ptr.To(int32(11901))}},
		Endpoints: []discoveryv1.Endpoint{
			{Addresses: []string{"10.0.0.1"}, Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true)}},
			{Addresses: []string{"10.0.0.2"}, Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(false)}},
			{Addresses: []string{"2001:db8::1"}},
		},
	}
	duplicate := slice.DeepCopy()
	duplicate.Name = "store-b"
	otherNamespace := slice.DeepCopy()
	otherNamespace.Namespace = "other"
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(slice, duplicate, otherNamespace).Build()
	r := &ThanosQueryReconciler{Client: c, handler: handlers.NewHandler(c, scheme, logr.Discard())}
	endpoints, err := r.resolveTLSFanout(context.Background(), query.Endpoint{ServiceName: "store", Namespace: "metrics", Port: 10901, Type: manifests.RegularLabel})
	require.NoError(t, err)
	require.Len(t, endpoints, 2)
	require.Equal(t, "10.0.0.1:11901", endpoints[0].Address)
	require.Equal(t, "[2001:db8::1]:11901", endpoints[1].Address)
	for _, ep := range endpoints {
		require.Equal(t, "store", ep.ServiceName)
		require.Equal(t, "metrics", ep.Namespace)
		require.Equal(t, manifests.RegularLabel, ep.Type)
	}
	require.NoError(t, c.Delete(context.Background(), slice))
	require.NoError(t, c.Delete(context.Background(), duplicate))
	endpoints, err = r.resolveTLSFanout(context.Background(), query.Endpoint{ServiceName: "store", Namespace: "metrics", Type: manifests.RegularLabel})
	require.NoError(t, err)
	require.Empty(t, endpoints)
}
