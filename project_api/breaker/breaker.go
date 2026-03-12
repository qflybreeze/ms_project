package breaker

import (
	"context"
	"time"

	"github.com/sony/gobreaker"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// 传递 gRPC 调用的原始错误
type circuitBreakerResult struct {
	err error
}

// 熔断策略（三态模型）：
//
//	Closed（正常）→连续请求中失败率≥60% 且请求数≥5
//	Open（熔断）→等待 10 秒→Half-Open（半开，放行1个探测请求）
//	Half-Open→半开探测，根据结果决定是否恢复Closed状态
func NewCircuitBreaker(name string) grpc.UnaryClientInterceptor {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        name,
		MaxRequests: 1,                // Half-Open 状态下允许通过的探测请求数
		Interval:    30 * time.Second, // Closed 状态下的统计周期，每30秒重置计数
		Timeout:     10 * time.Second, // Open → Half-Open 的等待时间
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// 至少 5 次请求，且失败率 ≥ 60% 时触发熔断
			return counts.Requests >= 5 &&
				float64(counts.TotalFailures)/float64(counts.Requests) >= 0.6
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			zap.L().Warn("circuit breaker state changed",
				zap.String("breaker", name),
				zap.String("from", from.String()),
				zap.String("to", to.String()),
			)
		},
	})

	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {

		result, cbErr := cb.Execute(func() (interface{}, error) {
			// 执行gRPC调用
			err := invoker(ctx, method, req, reply, cc, opts...)
			if err != nil && isServerError(err) {
				return nil, err
			}
			return &circuitBreakerResult{err: err}, nil
		})

		// 熔断器本身拒绝了请求（熔断中或半开限流中）
		if cbErr != nil {
			if cbErr == gobreaker.ErrOpenState || cbErr == gobreaker.ErrTooManyRequests {
				zap.L().Error("circuit breaker rejected request",
					zap.String("method", method),
				)
				return status.Error(codes.Unavailable, "服务暂时不可用，请稍后重试")
			}
			// 服务端错误，原样返回
			return cbErr
		}

		// 从 result 中取出业务错误（可能为nil，表示调用成功）
		if r, ok := result.(*circuitBreakerResult); ok {
			return r.err
		}
		return nil
	}
}

// 判断是否为服务端错误
// Unavailable / DeadlineExceeded / Internal / ResourceExhausted服务端问题计入
// InvalidArgument / NotFound / PermissionDenied 等业务/客户端问题不计入
func isServerError(err error) bool {
	s, ok := status.FromError(err)
	if !ok {
		return true // 未知错误，保守计入
	}
	switch s.Code() {
	case codes.Unavailable, codes.DeadlineExceeded, codes.Internal, codes.ResourceExhausted:
		return true
	default:
		return false
	}
}
