package grpcserver

import (
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"flightbooking/internal/flight/ferr"
)

func TestStatusError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code codes.Code
	}{
		{name: "invalid", err: ferr.Invalid("bad"), code: codes.InvalidArgument},
		{name: "missing", err: ferr.NotFound("нет"), code: codes.NotFound},
		{name: "seats", err: ferr.Exhausted("мало"), code: codes.ResourceExhausted},
		{name: "status", err: ferr.Precondition("нельзя"), code: codes.FailedPrecondition},
		{name: "duplicate", err: ferr.AlreadyExists("есть"), code: codes.AlreadyExists},
		{name: "unknown", err: errors.New("db"), code: codes.Internal},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := status.FromError(statusError(test.err))
			if !ok || got.Code() != test.code {
				t.Fatalf("got %v", statusError(test.err))
			}
		})
	}
}
