package grpcserver

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"flightbooking/internal/flight/ferr"
	"flightbooking/internal/flight/store"
	flightv1 "flightbooking/internal/flightv1"
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

func TestUpdateFlight(t *testing.T) {
	departure := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
	arrival := time.Date(2026, 4, 1, 13, 0, 0, 0, time.UTC)
	catalog := &fakeCatalog{flight: store.Flight{
		ID:             "00000000-0000-4000-8000-000000000005",
		FlightNumber:   "SU1005",
		Origin:         "SVO",
		Destination:    "AER",
		DepartureAt:    departure,
		ArrivalAt:      arrival,
		TotalSeats:     100,
		AvailableSeats: 100,
		Price:          decimal.RequireFromString("8200.00"),
		Status:         "DEPARTED",
	}}
	server := New(catalog)
	resp, err := server.UpdateFlight(context.Background(), &flightv1.UpdateFlightRequest{
		Id:          catalog.flight.ID,
		Price:       "8200.00",
		Status:      flightv1.FlightStatus_FLIGHT_STATUS_DEPARTED,
		DepartureAt: timestamppb.New(departure),
		ArrivalAt:   timestamppb.New(arrival),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !catalog.called || resp.GetPrice() != "8200.00" || resp.GetStatus() != flightv1.FlightStatus_FLIGHT_STATUS_DEPARTED {
		t.Fatalf("called=%v resp=%v", catalog.called, resp)
	}

	catalog.called = false
	rejected := []*flightv1.UpdateFlightRequest{
		{Id: "bad", Price: "8200.00", Status: flightv1.FlightStatus_FLIGHT_STATUS_DEPARTED, DepartureAt: timestamppb.New(departure), ArrivalAt: timestamppb.New(arrival)},
		{Id: catalog.flight.ID, Price: "0", Status: flightv1.FlightStatus_FLIGHT_STATUS_DEPARTED, DepartureAt: timestamppb.New(departure), ArrivalAt: timestamppb.New(arrival)},
		{Id: catalog.flight.ID, Price: "8200.00", Status: flightv1.FlightStatus_FLIGHT_STATUS_UNSPECIFIED, DepartureAt: timestamppb.New(departure), ArrivalAt: timestamppb.New(arrival)},
		{Id: catalog.flight.ID, Price: "8200.00", Status: flightv1.FlightStatus_FLIGHT_STATUS_DEPARTED, ArrivalAt: timestamppb.New(arrival)},
		{Id: catalog.flight.ID, Price: "8200.00", Status: flightv1.FlightStatus_FLIGHT_STATUS_DEPARTED, DepartureAt: timestamppb.New(arrival), ArrivalAt: timestamppb.New(departure)},
	}
	for _, req := range rejected {
		_, err := server.UpdateFlight(context.Background(), req)
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("запрос %+v: %v", req, err)
		}
	}
	if catalog.called {
		t.Fatal("невалидный запрос дошёл до хранилища")
	}
}

type fakeCatalog struct {
	flight store.Flight
	called bool
}

func (f *fakeCatalog) Search(context.Context, string, string, *time.Time) ([]store.Flight, error) {
	panic("search")
}

func (f *fakeCatalog) Get(context.Context, string) (store.Flight, error) {
	panic("get")
}

func (f *fakeCatalog) Reserve(context.Context, string, int, string) (store.Reservation, error) {
	panic("reserve")
}

func (f *fakeCatalog) Release(context.Context, string) (store.Reservation, error) {
	panic("release")
}

func (f *fakeCatalog) Update(context.Context, string, decimal.Decimal, string, time.Time, time.Time) (store.Flight, error) {
	f.called = true
	return f.flight, nil
}
