package flightclient

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var retryWaits = []time.Duration{100 * time.Millisecond, 200 * time.Millisecond}

func UnaryRetryInterceptor(attemptTimeout time.Duration) grpc.UnaryClientInterceptor {
	return newRetry(attemptTimeout, sleepContext)
}

func newRetry(attemptTimeout time.Duration, sleep func(context.Context, time.Duration) error) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		var err error
		for attempt := 0; attempt < 3; attempt++ {
			if attempt > 0 {
				if err := sleep(ctx, retryWaits[attempt-1]); err != nil {
					return status.FromContextError(err).Err()
				}
			}
			err = invokeOnce(ctx, attemptTimeout, method, req, reply, cc, invoker, opts...)
			if !transportFailure(err) {
				return err
			}
		}
		return err
	}
}

func invokeOnce(ctx context.Context, timeout time.Duration, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return invoker(ctx, method, req, reply, cc, opts...)
}

func sleepContext(ctx context.Context, wait time.Duration) error {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func transportFailure(err error) bool {
	switch status.Code(err) {
	case codes.Unavailable, codes.DeadlineExceeded:
		return true
	default:
		return false
	}
}
