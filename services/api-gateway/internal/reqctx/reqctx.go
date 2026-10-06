// Package reqctx хранит данные запроса (трассировка, пользователь) и переносит их в metadata gRPC-вызовов.
package reqctx

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/maltira/chavo-project-backend/proto/grpcx"
)

type Info struct {
	RequestID string
	TraceID   string
	// TraceParent — span Gateway, уходит в сервисы.
	TraceParent string
	// UserID и SessionID заполняются только после проверки access token.
	UserID    string
	SessionID string
}

type key struct{}

func With(ctx context.Context, info *Info) context.Context {
	return context.WithValue(ctx, key{}, info)
}

func From(ctx context.Context) *Info {
	if info, ok := ctx.Value(key{}).(*Info); ok {
		return info
	}
	return &Info{}
}

// ClientInterceptor собирает metadata вызова только из Info (заголовки клиента не пересылаются)
// и ставит дедлайн, если его нет.
func ClientInterceptor(timeout time.Duration) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		info := From(ctx)
		var pairs []string
		if info.RequestID != "" {
			pairs = append(pairs, grpcx.MDRequestID, info.RequestID)
		}
		if info.TraceParent != "" {
			pairs = append(pairs, grpcx.MDTraceParent, info.TraceParent)
		}
		if info.UserID != "" {
			pairs = append(pairs, grpcx.MDUserID, info.UserID)
		}
		ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(pairs...))

		if _, ok := ctx.Deadline(); !ok {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
