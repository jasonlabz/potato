package grpcx

import (
	"context"
	"fmt"
	"time"

	"github.com/jasonlabz/potato/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// unaryLoggingInterceptor 记录一元调用的请求和响应日志
func unaryLoggingInterceptor(logger *log.LoggerWrapper) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		start := time.Now()
		logger.Info(ctx, fmt.Sprintf("[gRPC] >>> method=%s target=%s", method, cc.Target()))

		err := invoker(ctx, method, req, reply, cc, opts...)

		latency := time.Since(start)
		code := codes.OK
		if err != nil {
			code = status.Code(err)
			logger.Error(ctx, fmt.Sprintf("[gRPC] <<< method=%s code=%s latency=%v error=%v", method, code, latency, err))
		} else {
			logger.Info(ctx, fmt.Sprintf("[gRPC] <<< method=%s code=%s latency=%v", method, code, latency))
		}
		return err
	}
}

// streamLoggingInterceptor 记录流式调用的请求日志
func streamLoggingInterceptor(logger *log.LoggerWrapper) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		start := time.Now()
		logger.Info(ctx, fmt.Sprintf("[gRPC] >>> stream method=%s target=%s", method, cc.Target()))

		s, err := streamer(ctx, desc, cc, method, opts...)

		latency := time.Since(start)
		if err != nil {
			code := status.Code(err)
			logger.Error(ctx, fmt.Sprintf("[gRPC] <<< stream method=%s code=%s latency=%v error=%v", method, code, latency, err))
		} else {
			logger.Info(ctx, fmt.Sprintf("[gRPC] <<< stream method=%s latency=%v (streaming)", method, latency))
		}
		return s, err
	}
}

// unaryRecoveryInterceptor 一元调用 panic 恢复
func unaryRecoveryInterceptor(logger *log.LoggerWrapper) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) (err error) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error(ctx, fmt.Sprintf("[gRPC] panic recovered in method=%s: %v", method, r))
				err = status.Errorf(codes.Internal, "client panic: %v", r)
			}
		}()
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// streamRecoveryInterceptor 流式调用 panic 恢复
func streamRecoveryInterceptor(logger *log.LoggerWrapper) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (s grpc.ClientStream, err error) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error(ctx, fmt.Sprintf("[gRPC] panic recovered in stream method=%s: %v", method, r))
				err = status.Errorf(codes.Internal, "client panic: %v", r)
			}
		}()
		return streamer(ctx, desc, cc, method, opts...)
	}
}
