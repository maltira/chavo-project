package grpcserver

import (
	"context"
	"net/http"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/maltira/chavo-project-backend/proto/grpcx"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/apperror"
)

// UnaryInterceptor: перевод доменных ошибок в gRPC status, recovery и лог вызова с request_id.
func UnaryInterceptor(log *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		start := time.Now()
		requestID, traceParent := grpcx.IncomingTrace(ctx)
		fields := []zap.Field{
			zap.String("method", info.FullMethod),
			zap.String("request_id", requestID),
			zap.String("traceparent", traceParent),
			zap.String("user_id", grpcx.IncomingUserID(ctx)),
		}

		defer func() {
			if r := recover(); r != nil {
				log.Error("Panic in gRPC handler", append(fields, zap.Any("panic", r))...)
				err = status.Error(codes.Internal, apperror.UserMessage(nil))
			}
			code := status.Code(err)
			log.Info("gRPC call", append(fields, zap.String("code", code.String()), zap.Duration("latency", time.Since(start)))...)
		}()

		resp, err = handler(ctx, req)
		if err != nil {
			if _, isStatus := status.FromError(err); !isStatus {
				if apperror.HTTPCode(err) == http.StatusInternalServerError {
					log.Error("Internal error", append(fields, zap.Error(err))...)
				}
				err = apperror.GRPCStatus(err)
			}
		}
		return resp, err
	}
}
