package flightclient

import (
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"flightbooking/internal/apperr"
)

func TestMapErrCircuitOpen(t *testing.T) {
	err := mapErr(status.Error(codes.Unavailable, circuitOpenMessage))
	target, ok := apperr.As(err)
	if !ok || target.Status != 503 || target.Code != "SERVICE_UNAVAILABLE" {
		t.Fatalf("%v", err)
	}
	err = mapErr(status.Error(codes.Unavailable, "connection refused"))
	target, ok = apperr.As(err)
	if !ok || target.Status != 503 || target.Code != "UNAVAILABLE" {
		t.Fatalf("%v", err)
	}
}
