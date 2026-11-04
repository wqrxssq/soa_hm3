package svcjwt

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestInterceptorsRoundTrip(t *testing.T) {
	secret := "secret"
	original := currentTime
	t.Cleanup(func() { currentTime = original })
	instant := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	var calls int
	currentTime = func() time.Time {
		calls++
		return instant.Add(time.Duration(calls) * time.Second)
	}

	var tokens []string
	client := UnaryClientInterceptor(secret)
	for range 2 {
		err := client(context.Background(), "/flight.v1.FlightService/GetFlight", nil, nil, nil,
			func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
				md, ok := metadata.FromOutgoingContext(ctx)
				if !ok {
					t.Fatal("нет metadata")
				}
				incoming := metadata.NewIncomingContext(context.Background(), md)
				_, err := UnaryServerInterceptor(secret)(incoming, nil, &grpc.UnaryServerInfo{},
					func(context.Context, any) (any, error) { return "ok", nil })
				tokens = append(tokens, md.Get(MetadataKey)[0])
				return err
			})
		if err != nil {
			t.Fatal(err)
		}
	}
	if tokens[0] == tokens[1] {
		t.Fatal("повторный вызов подписал тот же токен")
	}
}

func TestServerInterceptorRejects(t *testing.T) {
	secret := "secret"
	valid, err := Sign(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	expired, err := Sign(secret, time.Now().Add(-10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		ctx  context.Context
	}{
		{name: "missing", ctx: context.Background()},
		{name: "prefix", ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(MetadataKey, "Token "+valid))},
		{name: "garbage", ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(MetadataKey, "Bearer not-a-jwt"))},
		{name: "expired", ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(MetadataKey, "Bearer "+expired))},
	}
	interceptor := UnaryServerInterceptor(secret)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := interceptor(test.ctx, nil, &grpc.UnaryServerInfo{}, func(context.Context, any) (any, error) {
				return "ok", nil
			})
			if status.Code(err) != codes.Unauthenticated {
				t.Fatalf("got %v", err)
			}
		})
	}
}
