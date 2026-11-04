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
	interceptor := newRetry(func(_ context.Context, wait time.Duration) error {
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
	interceptor := newRetry(func(context.Context, time.Duration) error { return nil })
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
			interceptor := newRetry(func(context.Context, time.Duration) error {
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
	interceptor := newRetry(func(ctx context.Context, _ time.Duration) error {
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
