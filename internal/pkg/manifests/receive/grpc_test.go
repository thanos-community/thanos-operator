package receive

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	_ "google.golang.org/grpc/balancer/roundrobin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
	"gotest.tools/v3/assert"
)

func TestRouterGRPCServiceConfig(t *testing.T) {
	var serviceConfig string
	for _, arg := range routerArgsFrom(RouterOptions{}) {
		if config, ok := strings.CutPrefix(arg, "--receive.grpc-service-config="); ok {
			serviceConfig = config
			break
		}
	}
	assert.Assert(t, serviceConfig != "")

	// Reject ignored top-level fields, including the old misplaced retryPolicy.
	var config struct {
		LoadBalancingPolicy string          `json:"loadBalancingPolicy"`
		MethodConfig        json.RawMessage `json:"methodConfig"`
	}
	decoder := json.NewDecoder(strings.NewReader(serviceConfig))
	decoder.DisallowUnknownFields()
	assert.NilError(t, decoder.Decode(&config))
	assert.Equal(t, config.LoadBalancingPolicy, "round_robin")

	for _, tc := range []struct {
		name         string
		firstCode    codes.Code
		retryCode    codes.Code
		wantCode     codes.Code
		wantAttempts int32
	}{
		{
			name:         "success",
			firstCode:    codes.OK,
			wantCode:     codes.OK,
			wantAttempts: 1,
		},
		{
			name:         "unavailable_then_success",
			firstCode:    codes.Unavailable,
			retryCode:    codes.OK,
			wantCode:     codes.OK,
			wantAttempts: 2,
		},
		{
			name:         "persistent_unavailable",
			firstCode:    codes.Unavailable,
			retryCode:    codes.Unavailable,
			wantCode:     codes.Unavailable,
			wantAttempts: 2,
		},
		{
			name:         "conflict",
			firstCode:    codes.AlreadyExists,
			wantCode:     codes.AlreadyExists,
			wantAttempts: 1,
		},
		{
			name:         "internal_error",
			firstCode:    codes.Internal,
			wantCode:     codes.Internal,
			wantAttempts: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var attempts atomic.Int32
			listener := bufconn.Listen(1024 * 1024)
			server := grpc.NewServer()
			server.RegisterService(&grpc.ServiceDesc{
				ServiceName: "thanos.WriteableStore",
				HandlerType: (*interface{})(nil),
				Methods: []grpc.MethodDesc{{
					MethodName: "RemoteWrite",
					Handler: func(_ interface{}, _ context.Context, decode func(interface{}) error, _ grpc.UnaryServerInterceptor) (interface{}, error) {
						if err := decode(&emptypb.Empty{}); err != nil {
							return nil, err
						}
						code := tc.firstCode
						if attempts.Add(1) > 1 {
							code = tc.retryCode
						}
						if code == codes.OK {
							return &emptypb.Empty{}, nil
						}
						return nil, status.Error(code, "ingester rejected write")
					},
				}},
			}, struct{}{})
			serveDone := make(chan error, 1)
			go func() { serveDone <- server.Serve(listener) }()
			t.Cleanup(func() {
				server.Stop()
				assert.NilError(t, <-serveDone)
			})

			conn, err := grpc.NewClient("passthrough:///ingester",
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
					return listener.DialContext(ctx)
				}),
				grpc.WithDefaultServiceConfig(serviceConfig),
			)
			assert.NilError(t, err)
			t.Cleanup(func() { assert.NilError(t, conn.Close()) })

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			err = conn.Invoke(ctx, "/thanos.WriteableStore/RemoteWrite", &emptypb.Empty{}, &emptypb.Empty{})
			assert.Equal(t, status.Code(err), tc.wantCode)
			assert.Equal(t, attempts.Load(), tc.wantAttempts)

			policy := conn.GetMethodConfig("/thanos.WriteableStore/RemoteWrite").RetryPolicy
			assert.Assert(t, policy != nil)
			assert.Equal(t, policy.MaxAttempts, 2)
			assert.Equal(t, policy.InitialBackoff, 100*time.Millisecond)
			assert.Equal(t, policy.MaxBackoff, 100*time.Millisecond)
			assert.Equal(t, policy.BackoffMultiplier, float64(1))
			assert.DeepEqual(t, policy.RetryableStatusCodes, map[codes.Code]bool{codes.Unavailable: true})
			assert.Assert(t, conn.GetMethodConfig("/thanos.WriteableStore/Other").RetryPolicy == nil)
			assert.Assert(t, conn.GetMethodConfig("/thanos.Store/Series").RetryPolicy == nil)
		})
	}
}
