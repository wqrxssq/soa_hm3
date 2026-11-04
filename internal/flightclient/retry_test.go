package flightclient

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRetryStopsAfterThreeTransportFailures(t *testing.T) {
	var waits []time.Duration
	var calls int
	interceptor := newRetry(time.Second, func(_ context.Context, wait time.Duration) error {
		waits = append(waits, wait)
		return nil
	})
	err := interceptor(context.Background(), "/flight.v1.FlightService/GetFlight", nil, nil, nil,
		func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
			calls++
			return status.Error(codes.Unavailable, "down")
		})
	if status.Code(err) != codes.Unavailable || calls != 3 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
	if len(waits) != 2 || waits[0] != 100*time.Millisecond || waits[1] != 200*time.Millisecond {
		t.Fatalf("паузы %v", waits)
	}
}

func TestRetryReturnsOnSecondSuccess(t *testing.T) {
	var calls int
	interceptor := newRetry(time.Second, func(context.Context, time.Duration) error { return nil })
	err := interceptor(context.Background(), "/m", nil, nil, nil,
		func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
			calls++
			if calls == 1 {
				return status.Error(codes.DeadlineExceeded, "slow")
			}
			return nil
		})
	if err != nil || calls != 2 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestRetryCutsHungAttemptByTimeout(t *testing.T) {
	var calls int
	interceptor := newRetry(20*time.Millisecond, func(context.Context, time.Duration) error { return nil })
	started := time.Now()
	err := interceptor(context.Background(), "/m", nil, nil, nil,
		func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
			calls++
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("у попытки нет дедлайна")
			}
			if calls == 1 {
				<-ctx.Done()
				return status.FromContextError(ctx.Err()).Err()
			}
			return nil
		})
	if err != nil || calls != 2 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("попытка не прервана: %v", elapsed)
	}
}

func TestRetrySkipsBusinessErrors(t *testing.T) {
	codesToKeep := []codes.Code{
		codes.InvalidArgument,
		codes.NotFound,
		codes.ResourceExhausted,
		codes.FailedPrecondition,
		codes.AlreadyExists,
		codes.Unauthenticated,
	}
	for _, code := range codesToKeep {
		t.Run(code.String(), func(t *testing.T) {
			var calls int
			interceptor := newRetry(time.Second, func(context.Context, time.Duration) error {
				t.Fatal("пауза перед бизнес-ошибкой")
				return nil
			})
			err := interceptor(context.Background(), "/m", nil, nil, nil,
				func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
					calls++
					return status.Error(code, "no")
				})
			if status.Code(err) != code || calls != 1 {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
		})
	}
}

func TestRetryStopsWhenContextEnds(t *testing.T) {
	interceptor := newRetry(time.Second, func(ctx context.Context, _ time.Duration) error {
		return context.Canceled
	})
	var calls int
	err := interceptor(context.Background(), "/m", nil, nil, nil,
		func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
			calls++
			return status.Error(codes.Unavailable, "down")
		})
	if status.Code(err) != codes.Canceled || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}
