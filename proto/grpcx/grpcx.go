// Package grpcx — общие для сервисов и Gateway соглашения gRPC: ключи metadata и формат ошибок.
package grpcx

import (
	"context"
	"fmt"
	"net/http"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	// MDUserID ставит только Gateway из проверенного access token.
	MDUserID      = "x-user-id"
	MDRequestID   = "x-request-id"
	MDTraceParent = "traceparent"

	errorDomain = "chavo"
	metaMessage = "message"
)

// Error — gRPC-ошибка с машинным reason и текстом для пользователя.
func Error(code codes.Code, reason, message string) error {
	st := status.New(code, message)
	if withInfo, err := st.WithDetails(&errdetails.ErrorInfo{
		Reason:   reason,
		Domain:   errorDomain,
		Metadata: map[string]string{metaMessage: message},
	}); err == nil {
		st = withInfo
	}
	return st.Err()
}

// Details разбирает ошибку gRPC-вызова; message пуст, если сервис его не передал.
func Details(err error) (code codes.Code, reason, message string) {
	st, ok := status.FromError(err)
	if !ok {
		return codes.Unknown, "", ""
	}
	for _, d := range st.Details() {
		if info, ok := d.(*errdetails.ErrorInfo); ok && info.GetDomain() == errorDomain {
			return st.Code(), info.GetReason(), info.GetMetadata()[metaMessage]
		}
	}
	return st.Code(), "", ""
}

// CodeFromHTTP переводит HTTP-статус из таблиц apperror сервисов в gRPC-код.
func CodeFromHTTP(httpStatus int) codes.Code {
	switch httpStatus {
	case http.StatusBadRequest:
		return codes.InvalidArgument
	case http.StatusUnauthorized:
		return codes.Unauthenticated
	case http.StatusForbidden:
		return codes.PermissionDenied
	case http.StatusNotFound:
		return codes.NotFound
	case http.StatusConflict:
		return codes.FailedPrecondition
	case http.StatusTooManyRequests:
		return codes.ResourceExhausted
	case http.StatusServiceUnavailable:
		return codes.Unavailable
	default:
		return codes.Internal
	}
}

// HTTPFromCode — обратное преобразование для Gateway.
func HTTPFromCode(code codes.Code) int {
	switch code {
	case codes.OK:
		return http.StatusOK
	case codes.InvalidArgument, codes.OutOfRange:
		return http.StatusBadRequest
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists, codes.FailedPrecondition, codes.Aborted:
		return http.StatusConflict
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}

func incoming(ctx context.Context, key string) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	if v := md.Get(key); len(v) > 0 {
		return v[0]
	}
	return ""
}

// IncomingUserID — значение x-user-id входящего вызова ("" если нет).
func IncomingUserID(ctx context.Context) string { return incoming(ctx, MDUserID) }

// IncomingTrace — request_id и traceparent входящего вызова.
func IncomingTrace(ctx context.Context) (requestID, traceParent string) {
	return incoming(ctx, MDRequestID), incoming(ctx, MDTraceParent)
}

// PropagateTrace — клиентский интерсептор для вызовов сервис → сервис: переносит
// request_id и traceparent из входящего вызова в исходящий.
func PropagateTrace() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		requestID, traceParent := IncomingTrace(ctx)
		if requestID != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, MDRequestID, requestID)
		}
		if traceParent != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, MDTraceParent, traceParent)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// HealthProbe вызывает grpc.health.v1 по адресу; используется командой `healthcheck` бинарников в compose.
func HealthProbe(ctx context.Context, addr string) error {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()
	resp, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		return err
	}
	if resp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		return fmt.Errorf("status %s", resp.GetStatus())
	}
	return nil
}
