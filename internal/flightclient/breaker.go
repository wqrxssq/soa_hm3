package flightclient

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const circuitOpenMessage = "circuit breaker open"

type breakerState int

const (
	stateClosed breakerState = iota
	stateOpen
	stateHalfOpen
)

func (s breakerState) String() string {
	switch s {
	case stateClosed:
		return "CLOSED"
	case stateOpen:
		return "OPEN"
	case stateHalfOpen:
		return "HALF_OPEN"
	default:
		return ""
	}
}

type breaker struct {
	mu          sync.Mutex
	state       breakerState
	failures    int
	threshold   int
	timeout     time.Duration
	openedUntil time.Time
	log         *slog.Logger
	now         func() time.Time
}

func UnaryBreakerInterceptor(threshold int, timeout time.Duration, log *slog.Logger) grpc.UnaryClientInterceptor {
	if log == nil {
		log = slog.Default()
	}
	return newBreaker(threshold, timeout, log, time.Now).intercept
}

func newBreaker(threshold int, timeout time.Duration, log *slog.Logger, now func() time.Time) *breaker {
	return &breaker{threshold: threshold, timeout: timeout, log: log, now: now}
}

func (b *breaker) intercept(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	if err := b.before(); err != nil {
		return err
	}
	err := invoker(ctx, method, req, reply, cc, opts...)
	b.after(err)
	return err
}

func (b *breaker) before() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch b.state {
	case stateClosed:
		return nil
	case stateOpen:
		if b.now().Before(b.openedUntil) {
			return status.Error(codes.Unavailable, circuitOpenMessage)
		}
		b.transition(stateOpen, stateHalfOpen)
		return nil
	default:
		return status.Error(codes.Unavailable, circuitOpenMessage)
	}
}

func (b *breaker) after(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	failed := transportFailure(err)
	switch b.state {
	case stateClosed:
		if !failed {
			b.failures = 0
			return
		}
		b.failures++
		if b.failures >= b.threshold {
			b.failures = 0
			b.openedUntil = b.now().Add(b.timeout)
			b.transition(stateClosed, stateOpen)
		}
	case stateHalfOpen:
		if failed {
			b.openedUntil = b.now().Add(b.timeout)
			b.transition(stateHalfOpen, stateOpen)
			return
		}
		b.failures = 0
		b.transition(stateHalfOpen, stateClosed)
	}
}

func (b *breaker) transition(from, to breakerState) {
	b.state = to
	b.log.Info("circuit breaker", "transition", from.String()+" → "+to.String())
}
