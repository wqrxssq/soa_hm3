package rules

import (
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

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

func TestMatchExistingReserve(t *testing.T) {
	reuse, err := MatchExistingReserve("ACTIVE", "flight", "flight", 2, 2)
	if err != nil || !reuse {
		t.Fatalf("повтор той же резервации: %v %v", reuse, err)
	}
	tests := []struct {
		name     string
		status   string
		flightID string
		seats    int
		kind     ferr.Kind
	}{
		{name: "other flight", status: "ACTIVE", flightID: "other", seats: 2, kind: ferr.KindAlreadyExists},
		{name: "other seats", status: "ACTIVE", flightID: "flight", seats: 1, kind: ferr.KindAlreadyExists},
		{name: "released", status: "RELEASED", flightID: "flight", seats: 2, kind: ferr.KindPrecondition},
		{name: "expired", status: "EXPIRED", flightID: "flight", seats: 2, kind: ferr.KindPrecondition},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reuse, err := MatchExistingReserve(test.status, "flight", test.flightID, 2, test.seats)
			var target *ferr.Error
			if reuse || !errors.As(err, &target) || target.Kind != test.kind {
				t.Fatalf("reuse=%v err=%v", reuse, err)
			}
		})
	}
}

func TestDepartureDayUsesUTC(t *testing.T) {
	utc := time.Date(2026, 4, 1, 23, 30, 0, 0, time.UTC)
	moscow := time.Date(2026, 4, 2, 2, 30, 0, 0, time.FixedZone("MSK", 3*60*60))
	if !DepartureDay(utc).Equal(DepartureDay(moscow)) {
		t.Fatalf("%s и %s разошлись", DepartureDay(utc), DepartureDay(moscow))
	}
	next := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
	if DepartureDay(utc).Equal(DepartureDay(next)) {
		t.Fatal("следующий день совпал")
	}
}

func TestValidateUpdate(t *testing.T) {
	current := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
	departure := time.Date(2026, 4, 1, 9, 15, 0, 0, time.UTC)
	arrival := time.Date(2026, 4, 1, 13, 0, 0, 0, time.UTC)
	price := decimal.RequireFromString("10.50")
	if err := ValidateUpdate(current, departure, arrival, price, "SCHEDULED"); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		departure time.Time
		arrival   time.Time
		price     decimal.Decimal
		status    string
	}{
		{name: "other day", departure: departure.Add(24 * time.Hour), arrival: arrival.Add(24 * time.Hour), price: price, status: "SCHEDULED"},
		{name: "arrival", departure: arrival, arrival: departure, price: price, status: "SCHEDULED"},
		{name: "price", departure: departure, arrival: arrival, price: decimal.Zero, status: "SCHEDULED"},
		{name: "status", departure: departure, arrival: arrival, price: price, status: "BOARDING"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateUpdate(current, test.departure, test.arrival, test.price, test.status)
			var target *ferr.Error
			if !errors.As(err, &target) || target.Kind != ferr.KindInvalid {
				t.Fatal(err)
			}
		})
	}
}
