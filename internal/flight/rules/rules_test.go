package rules

import (
	"errors"
	"testing"

	"flightbooking/internal/flight/ferr"
)

func TestEvaluateReserve(t *testing.T) {
	tests := []struct {
		name      string
		status    string
		available int
		requested int
		kind      ferr.Kind
	}{
		{name: "last seat", status: "SCHEDULED", available: 1, requested: 1},
		{name: "not scheduled", status: "DEPARTED", available: 10, requested: 1, kind: ferr.KindPrecondition},
		{name: "cancelled", status: "CANCELLED", available: 10, requested: 1, kind: ferr.KindPrecondition},
		{name: "completed", status: "COMPLETED", available: 10, requested: 1, kind: ferr.KindPrecondition},
		{name: "not enough", status: "SCHEDULED", available: 5, requested: 6, kind: ferr.KindExhausted},
		{name: "zero seats", status: "SCHEDULED", available: 5, requested: 0, kind: ferr.KindInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := EvaluateReserve(test.status, test.available, test.requested)
			if test.kind == 0 {
				if err != nil {
					t.Fatalf("expected success, got %v", err)
				}
				return
			}
			var target *ferr.Error
			if !errors.As(err, &target) || target.Kind != test.kind {
				t.Fatalf("got %v, want kind %d", err, test.kind)
			}
		})
	}
}

func TestEvaluateRelease(t *testing.T) {
	action, err := EvaluateRelease("ACTIVE")
	if err != nil || action != ReturnSeats {
		t.Fatalf("active: %s %v", action, err)
	}
	action, err = EvaluateRelease("RELEASED")
	if err != nil || action != Keep {
		t.Fatalf("released: %s %v", action, err)
	}
	_, err = EvaluateRelease("EXPIRED")
	var target *ferr.Error
	if !errors.As(err, &target) || target.Kind != ferr.KindPrecondition {
		t.Fatalf("expired: %v", err)
	}
}
