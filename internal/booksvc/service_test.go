package booksvc

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"flightbooking/internal/apperr"
)

func TestTotalPrice(t *testing.T) {
	got := TotalPrice(2, decimal.RequireFromString("7200.00"))
	if !got.Equal(decimal.RequireFromString("14400.00")) {
		t.Fatalf("got %s", got)
	}
	got = TotalPrice(3, decimal.RequireFromString("10.10"))
	if !got.Equal(decimal.RequireFromString("30.30")) {
		t.Fatalf("got %s", got)
	}
}

func TestCreateReserveFailureDoesNotInsert(t *testing.T) {
	flights := &fakeFlights{
		flight:     scheduledFlight(),
		reserveErr: apperr.New(409, "RESOURCE_EXHAUSTED", "недостаточно мест"),
	}
	repo := &fakeRepo{}
	_, err := newService(flights, repo).Create(context.Background(), sampleInput())
	expectApp(t, err, 409, "RESOURCE_EXHAUSTED")
	if len(repo.inserted) != 0 || len(flights.released) != 0 {
		t.Fatalf("inserted %d released %d", len(repo.inserted), len(flights.released))
	}
}

func TestCreateMissingFlightDoesNotReserve(t *testing.T) {
	flights := &fakeFlights{getErr: apperr.New(404, "NOT_FOUND", "рейс не найден")}
	repo := &fakeRepo{}
	_, err := newService(flights, repo).Create(context.Background(), sampleInput())
	expectApp(t, err, 404, "NOT_FOUND")
	if flights.reserves != 0 || len(repo.inserted) != 0 {
		t.Fatal("reserve or insert was called")
	}
}

func TestCreateNotScheduledDoesNotReserve(t *testing.T) {
	flight := scheduledFlight()
	flight.Status = "DEPARTED"
	flights := &fakeFlights{flight: flight}
	repo := &fakeRepo{}
	_, err := newService(flights, repo).Create(context.Background(), sampleInput())
	expectApp(t, err, 409, "FAILED_PRECONDITION")
	if flights.reserves != 0 || len(repo.inserted) != 0 {
		t.Fatal("reserve or insert was called")
	}
}

func TestCreateSnapshotsPrice(t *testing.T) {
	flights := &fakeFlights{flight: scheduledFlight()}
	repo := &fakeRepo{}
	input := sampleInput()
	input.SeatCount = 2
	got, err := newService(flights, repo).Create(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.inserted) != 1 {
		t.Fatalf("inserts %d", len(repo.inserted))
	}
	want := decimal.RequireFromString("14400.00")
	if !repo.inserted[0].TotalPrice.Equal(want) || !got.TotalPrice.Equal(want) {
		t.Fatalf("price %s", got.TotalPrice)
	}
	if got.Status != "CONFIRMED" || flights.reserved[0] != got.ID {
		t.Fatalf("status %s id %s reserved %v", got.Status, got.ID, flights.reserved)
	}
}

func TestCreateReleasesReservationWhenInsertFails(t *testing.T) {
	flights := &fakeFlights{flight: scheduledFlight()}
	repo := &fakeRepo{insertErr: errors.New("db down")}
	_, err := newService(flights, repo).Create(context.Background(), sampleInput())
	expectApp(t, err, 500, "INTERNAL_ERROR")
	if len(flights.released) != 1 || flights.released[0] != flights.reserved[0] {
		t.Fatalf("reserved %v released %v", flights.reserved, flights.released)
	}
}

func TestCancelReleasesThenUpdates(t *testing.T) {
	flights := &fakeFlights{}
	repo := &fakeRepo{booking: confirmedBooking()}
	got, err := newService(flights, repo).Cancel(context.Background(), repo.booking.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(flights.released) != 1 || flights.released[0] != repo.booking.ID || !repo.cancelCalled {
		t.Fatal("cancel flow did not release and update")
	}
	if got.Status != "CANCELLED" {
		t.Fatalf("status %s", got.Status)
	}
}

func TestCancelRejectsTerminalStatus(t *testing.T) {
	booking := confirmedBooking()
	booking.Status = "CANCELLED"
	flights := &fakeFlights{}
	repo := &fakeRepo{booking: booking}
	_, err := newService(flights, repo).Cancel(context.Background(), booking.ID)
	expectApp(t, err, 409, "CONFLICT")
	if len(flights.released) != 0 || repo.cancelCalled {
		t.Fatal("release or cancel was called")
	}
}

func newService(flights *fakeFlights, repo *fakeRepo) *Service {
	return New(flights, repo, slog.New(slog.DiscardHandler))
}

func sampleInput() CreateInput {
	return CreateInput{
		UserID:         "00000000-0000-4000-8000-000000000010",
		FlightID:       "00000000-0000-4000-8000-000000000002",
		PassengerName:  "Ada Lovelace",
		PassengerEmail: "ada@example.com",
		SeatCount:      1,
	}
}

func scheduledFlight() Flight {
	return Flight{
		ID:             sampleInput().FlightID,
		FlightNumber:   "SU1002",
		Origin:         "LED",
		Destination:    "VKO",
		Status:         "SCHEDULED",
		Price:          decimal.RequireFromString("7200.00"),
		TotalSeats:     100,
		AvailableSeats: 100,
	}
}

func confirmedBooking() Booking {
	return Booking{
		ID:             "00000000-0000-4000-8000-000000000099",
		UserID:         sampleInput().UserID,
		FlightID:       sampleInput().FlightID,
		PassengerName:  "Ada Lovelace",
		PassengerEmail: "ada@example.com",
		SeatCount:      1,
		TotalPrice:     decimal.RequireFromString("7200.00"),
		Status:         "CONFIRMED",
	}
}

func expectApp(t *testing.T, err error, status int, code string) {
	t.Helper()
	target, ok := apperr.As(err)
	if !ok || target.Status != status || target.Code != code {
		t.Fatalf("got %v, want %d %s", err, status, code)
	}
}

type fakeFlights struct {
	flight     Flight
	getErr     error
	reserveErr error
	reserves   int
	reserved   []string
	released   []string
}

func (f *fakeFlights) Search(context.Context, string, string, *time.Time) ([]Flight, error) {
	return []Flight{f.flight}, nil
}

func (f *fakeFlights) Get(context.Context, string) (Flight, error) {
	if f.getErr != nil {
		return Flight{}, f.getErr
	}
	return f.flight, nil
}

func (f *fakeFlights) Reserve(_ context.Context, _ string, _ int, bookingID string) error {
	f.reserves++
	f.reserved = append(f.reserved, bookingID)
	return f.reserveErr
}

func (f *fakeFlights) Release(_ context.Context, bookingID string) error {
	f.released = append(f.released, bookingID)
	return nil
}

type fakeRepo struct {
	booking      Booking
	getErr       error
	insertErr    error
	cancelErr    error
	inserted     []NewBooking
	cancelCalled bool
}

func (f *fakeRepo) Insert(_ context.Context, in NewBooking) (Booking, error) {
	f.inserted = append(f.inserted, in)
	if f.insertErr != nil {
		return Booking{}, f.insertErr
	}
	return Booking{
		ID:             in.ID,
		UserID:         in.UserID,
		FlightID:       in.FlightID,
		PassengerName:  in.PassengerName,
		PassengerEmail: in.PassengerEmail,
		SeatCount:      in.SeatCount,
		TotalPrice:     in.TotalPrice,
		Status:         "CONFIRMED",
		CreatedAt:      time.Unix(1, 0).UTC(),
		UpdatedAt:      time.Unix(1, 0).UTC(),
	}, nil
}

func (f *fakeRepo) Get(context.Context, string) (Booking, error) {
	if f.getErr != nil {
		return Booking{}, f.getErr
	}
	if f.booking.ID == "" {
		return Booking{}, ErrNotFound
	}
	return f.booking, nil
}

func (f *fakeRepo) ListByUser(context.Context, string) ([]Booking, error) {
	return []Booking{}, nil
}

func (f *fakeRepo) Cancel(context.Context, string) (Booking, error) {
	f.cancelCalled = true
	if f.cancelErr != nil {
		return Booking{}, f.cancelErr
	}
	f.booking.Status = "CANCELLED"
	return f.booking, nil
}
