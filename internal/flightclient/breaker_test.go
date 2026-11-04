package flightclient

import (
	"bytes"
	"context"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"flightbooking/internal/svcjwt"
)

func TestBreakerOpensAfterConsecutiveFailures(t *testing.T) {
	now := time.Now()
	var logs bytes.Buffer
	var calls int
	br := newBreaker(5, time.Second, slog.New(slog.NewJSONHandler(&logs, nil)), func() time.Time { return now })
	fail := func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
		calls++
		return status.Error(codes.Unavailable, "down")
	}
	for range 4 {
		if err := br.intercept(context.Background(), "/m", nil, nil, nil, fail); status.Code(err) != codes.Unavailable {
			t.Fatal(err)
		}
	}
	if err := br.intercept(context.Background(), "/m", nil, nil, nil,
		func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
			calls++
			return status.Error(codes.NotFound, "нет")
		}); status.Code(err) != codes.NotFound {
		t.Fatal(err)
	}
	for range 4 {
		_ = br.intercept(context.Background(), "/m", nil, nil, nil, fail)
	}
	if calls != 9 {
		t.Fatalf("до открытия ушло %d вызовов", calls)
	}
	if bytes.Contains(logs.Bytes(), []byte("OPEN")) {
		t.Fatalf("breaker открылся рано: %s", logs.String())
	}
	_ = br.intercept(context.Background(), "/m", nil, nil, nil, fail)
	if !bytes.Contains(logs.Bytes(), []byte("CLOSED → OPEN")) {
		t.Fatalf("нет перехода: %s", logs.String())
	}
	before := calls
	err := br.intercept(context.Background(), "/m", nil, nil, nil, fail)
	if status.Code(err) != codes.Unavailable || status.Convert(err).Message() != circuitOpenMessage || calls != before {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestBreakerProbe(t *testing.T) {
	now := time.Now()
	var logs bytes.Buffer
	br := newBreaker(1, time.Second, slog.New(slog.NewJSONHandler(&logs, nil)), func() time.Time { return now })
	down := func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
		return status.Error(codes.DeadlineExceeded, "slow")
	}
	_ = br.intercept(context.Background(), "/m", nil, nil, nil, down)
	now = now.Add(time.Second)
	if err := br.intercept(context.Background(), "/m", nil, nil, nil, down); status.Code(err) != codes.DeadlineExceeded {
		t.Fatal(err)
	}
	if !bytes.Contains(logs.Bytes(), []byte("OPEN → HALF_OPEN")) || !bytes.Contains(logs.Bytes(), []byte("HALF_OPEN → OPEN")) {
		t.Fatalf("проба: %s", logs.String())
	}
	err := br.intercept(context.Background(), "/m", nil, nil, nil, func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
		t.Fatal("вызов ушёл до таймаута")
		return nil
	})
	if status.Convert(err).Message() != circuitOpenMessage {
		t.Fatal(err)
	}

	now = now.Add(time.Second)
	if err := br.intercept(context.Background(), "/m", nil, nil, nil, func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(logs.Bytes(), []byte("HALF_OPEN → CLOSED")) {
		t.Fatalf("закрытие: %s", logs.String())
	}
}

func TestHalfOpenAllowsOneProbe(t *testing.T) {
	now := time.Now()
	br := newBreaker(1, time.Minute, slog.New(slog.DiscardHandler), func() time.Time { return now })
	_ = br.intercept(context.Background(), "/m", nil, nil, nil, func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
		return status.Error(codes.Unavailable, "down")
	})
	now = now.Add(time.Minute)

	var calls atomic.Int32
	release := make(chan struct{})
	started := make(chan struct{})
	invoke := func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
		if calls.Add(1) == 1 {
			close(started)
			<-release
		}
		return nil
	}
	errs := make(chan error, 2)
	go func() {
		errs <- br.intercept(context.Background(), "/m", nil, nil, nil, invoke)
	}()
	<-started
	go func() {
		errs <- br.intercept(context.Background(), "/m", nil, nil, nil, invoke)
	}()
	select {
	case err := <-errs:
		if status.Convert(err).Message() != circuitOpenMessage {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("второй вызов ждал пробу")
	}
	close(release)
	if err := <-errs; err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("проб %d", calls.Load())
	}
}

func TestBreakerCountsRetryAsOneCall(t *testing.T) {
	var calls int
	transport := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		calls++
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok || len(md.Get(svcjwt.MetadataKey)) != 1 {
			t.Fatal("нет токена")
		}
		if calls < 3 {
			return status.Error(codes.Unavailable, "down")
		}
		return nil
	}
	jwt := svcjwt.UnaryClientInterceptor("secret")
	retry := newRetry(time.Second, func(context.Context, time.Duration) error { return nil })
	br := newBreaker(1, time.Second, slog.New(slog.DiscardHandler), time.Now)
	call := func() error {
		return br.intercept(context.Background(), "/flight.v1.FlightService/GetFlight", nil, nil, nil,
			func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
				return retry(ctx, method, req, reply, cc,
					func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
						return jwt(ctx, method, req, reply, cc, transport, opts...)
					}, opts...)
			})
	}
	if err := call(); err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatalf("попыток %d", calls)
	}
	if br.state != stateClosed || br.failures != 0 {
		t.Fatalf("state=%s failures=%d", br.state, br.failures)
	}

	calls = 0
	transport = func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
		calls++
		return status.Error(codes.Unavailable, "down")
	}
	if err := call(); status.Code(err) != codes.Unavailable {
		t.Fatal(err)
	}
	if calls != 3 || br.state != stateOpen {
		t.Fatalf("calls=%d state=%s", calls, br.state)
	}
	before := calls
	err := call()
	if status.Convert(err).Message() != circuitOpenMessage || calls != before {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}
