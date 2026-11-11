package v1

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type LoggerInterceptor struct {
}

func NewLoggerInterceptor() *LoggerInterceptor {
	return &LoggerInterceptor{}
}

func (in *LoggerInterceptor) LoggerInterceptor(ctx context.Context, request any, serverInfo *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	resp, err := handler(ctx, request)
	in.loggerInterceptorDo(ctx, serverInfo.FullMethod, err)
	return resp, err
}

func (*LoggerInterceptor) loggerInterceptorDo(ctx context.Context, fullMethod string, err error) {
	if err == nil {
		return
	}

	st := status.Convert(err)
	switch st.Code() {
	case codes.OK, codes.Unauthenticated, codes.OutOfRange, codes.PermissionDenied, codes.NotFound:
		return
	}

	slog.LogAttrs(ctx, slog.LevelError, "grpc request failed",
		slog.String("method", fullMethod),
		slog.String("code", st.Code().String()),
		slog.String("error", err.Error()),
	)
}
