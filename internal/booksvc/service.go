package booksvc

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"flightbooking/internal/apperr"
)

var ErrNotFound = errors.New("not found")

type Flight struct {
	ID             string
	FlightNumber   string
	Origin         string
	Destination    string
	DepartureAt    time.Time
	ArrivalAt      time.Time
	TotalSeats     int
	AvailableSeats int
	Price          decimal.Decimal
	Status         string
}

type Booking struct {
	ID             string
	UserID         string
	FlightID       string
	PassengerName  string
	PassengerEmail string
	SeatCount      int
	TotalPrice     decimal.Decimal
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type NewBooking struct {
	ID             string
	UserID         string
	FlightID       string
	PassengerName  string
	PassengerEmail string
	SeatCount      int
	TotalPrice     decimal.Decimal
}

type CreateInput struct {
	UserID         string
	FlightID       string
	PassengerName  string
	PassengerEmail string
	SeatCount      int
}

type Flights interface {
	Search(ctx context.Context, origin, destination string, day *time.Time) ([]Flight, error)
	Get(ctx context.Context, id string) (Flight, error)
	Reserve(ctx context.Context, flightID string, seatCount int, bookingID string) error
	Release(ctx context.Context, bookingID string) error
}

type Repository interface {
	Insert(ctx context.Context, in NewBooking) (Booking, error)
	Get(ctx context.Context, id string) (Booking, error)
	ListByUser(ctx context.Context, userID string) ([]Booking, error)
	Cancel(ctx context.Context, id string) (Booking, error)
}

type Service struct {
	flights Flights
	repo    Repository
	log     *slog.Logger
}

func New(flights Flights, repo Repository, log *slog.Logger) *Service {
	return &Service{flights: flights, repo: repo, log: log}
}

func TotalPrice(seatCount int, price decimal.Decimal) decimal.Decimal {
	return price.Mul(decimal.NewFromInt(int64(seatCount))).Round(2)
}

func (s *Service) Search(ctx context.Context, origin, destination string, day *time.Time) ([]Flight, error) {
	flights, err := s.flights.Search(ctx, origin, destination, day)
	if err != nil {
		return nil, err
	}
	if flights == nil {
		return []Flight{}, nil
	}
	return flights, nil
}

func (s *Service) GetFlight(ctx context.Context, id string) (Flight, error) {
	return s.flights.Get(ctx, id)
}

func (s *Service) Create(ctx context.Context, in CreateInput) (Booking, error) {
	bookingID := uuid.NewString()
	flight, err := s.flights.Get(ctx, in.FlightID)
	if err != nil {
		return Booking{}, err
	}
	if flight.Status != "SCHEDULED" {
		return Booking{}, apperr.New(409, "FAILED_PRECONDITION", "рейс недоступен для бронирования")
	}
	if err := s.flights.Reserve(ctx, in.FlightID, in.SeatCount, bookingID); err != nil {
		return Booking{}, err
	}
	booking, err := s.repo.Insert(ctx, NewBooking{
		ID:             bookingID,
		UserID:         in.UserID,
		FlightID:       in.FlightID,
		PassengerName:  in.PassengerName,
		PassengerEmail: in.PassengerEmail,
		SeatCount:      in.SeatCount,
		TotalPrice:     TotalPrice(in.SeatCount, flight.Price),
	})
	if err != nil {
		if releaseErr := s.flights.Release(ctx, bookingID); releaseErr != nil {
			s.log.Error("compensate reservation", "booking_id", bookingID, "err", releaseErr)
		}
		return Booking{}, apperr.New(500, "INTERNAL_ERROR", "внутренняя ошибка")
	}
	return booking, nil
}

func (s *Service) GetBooking(ctx context.Context, id string) (Booking, error) {
	booking, err := s.repo.Get(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return Booking{}, apperr.New(404, "NOT_FOUND", "бронирование не найдено")
	}
	if err != nil {
		return Booking{}, apperr.New(500, "INTERNAL_ERROR", "внутренняя ошибка")
	}
	return booking, nil
}

func (s *Service) List(ctx context.Context, userID string) ([]Booking, error) {
	bookings, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, apperr.New(500, "INTERNAL_ERROR", "внутренняя ошибка")
	}
	if bookings == nil {
		return []Booking{}, nil
	}
	return bookings, nil
}

func (s *Service) Cancel(ctx context.Context, id string) (Booking, error) {
	booking, err := s.repo.Get(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return Booking{}, apperr.New(404, "NOT_FOUND", "бронирование не найдено")
	}
	if err != nil {
		return Booking{}, apperr.New(500, "INTERNAL_ERROR", "внутренняя ошибка")
	}
	if booking.Status != "CONFIRMED" {
		return Booking{}, apperr.New(409, "CONFLICT", "бронирование уже нельзя отменить")
	}
	if err := s.flights.Release(ctx, booking.ID); err != nil {
		return Booking{}, err
	}
	cancelled, err := s.repo.Cancel(ctx, booking.ID)
	if errors.Is(err, ErrNotFound) {
		return Booking{}, apperr.New(409, "CONFLICT", "бронирование уже нельзя отменить")
	}
	if err != nil {
		return Booking{}, apperr.New(500, "INTERNAL_ERROR", "внутренняя ошибка")
	}
	return cancelled, nil
}
