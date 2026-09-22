package interceptors

import (
	"context"
	"time"

	"github.com/Lhh220/g-video/logic-server/pkg/logx"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var grpcDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "grpc_server_handling_seconds",
	Help:    "gRPC 服务端一元调用耗时",
	Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
}, []string{"method", "code"})

// requestIDFromCtx 从入站 metadata 提取 web 层注入的 request_id
func requestIDFromCtx(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	if v := md.Get("x-request-id"); len(v) > 0 {
		return v[0]
	}
	return ""
}

// UnaryLog 访问日志 + 耗时指标：method/code/latency/request_id
func UnaryLog() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		latency := time.Since(start)
		code := status.Code(err)

		fields := []zap.Field{
			zap.String("method", info.FullMethod),
			zap.String("code", code.String()),
			zap.Duration("latency", latency),
			zap.String("request_id", requestIDFromCtx(ctx)),
		}
		if err != nil {
			fields = append(fields, zap.Error(err))
			logx.L().Warn("grpc_call", fields...)
		} else {
			logx.L().Info("grpc_call", fields...)
		}

		grpcDuration.WithLabelValues(info.FullMethod, code.String()).Observe(latency.Seconds())
		return resp, err
	}
}

// UnaryRecovery 兜底 panic：转为 Internal 错误返回，避免拖垮整个 gRPC 服务
func UnaryRecovery() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				logx.L().Error("grpc_panic",
					zap.Any("panic", r),
					zap.String("method", info.FullMethod),
					zap.String("request_id", requestIDFromCtx(ctx)),
				)
				err = status.Error(codes.Internal, "服务内部错误")
			}
		}()
		return handler(ctx, req)
	}
}
