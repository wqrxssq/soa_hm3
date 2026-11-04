package grpcserver

import (
	"context"
	"errors"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"flightbooking/internal/flight/ferr"
	"flightbooking/internal/flight/store"
	flightv1 "flightbooking/internal/flightv1"
)

var iataCode = regexp.MustCompile(`^[A-Z]{3}$`)

type catalog interface {
	Search(ctx context.Context, origin, destination string, day *time.Time) ([]store.Flight, error)
	Get(ctx context.Context, id string) (store.Flight, error)
	Reserve(ctx context.Context, flightID string, seatCount int, bookingID string) (store.Reservation, error)
	Release(ctx context.Context, bookingID string) (store.Reservation, error)
	Update(ctx context.Context, id string, price decimal.Decimal, status string, departure, arrival time.Time) (store.Flight, error)
}

type Server struct {
	flightv1.UnimplementedFlightServiceServer
	flights catalog
}

func New(flights catalog) *Server {
	return &Server{flights: flights}
}

func (s *Server) SearchFlights(ctx context.Context, req *flightv1.SearchFlightsRequest) (*flightv1.SearchFlightsResponse, error) {
	if !iataCode.MatchString(req.GetOrigin()) || !iataCode.MatchString(req.GetDestination()) {
		return nil, status.Error(codes.InvalidArgument, "маршрут задаётся кодами IATA")
	}
	if req.GetOrigin() == req.GetDestination() {
		return nil, status.Error(codes.InvalidArgument, "аэропорты вылета и прилёта совпадают")
	}
	flights, err := s.flights.Search(ctx, req.GetOrigin(), req.GetDestination(), utcDay(req.GetDate()))
	if err != nil {
		return nil, statusError(err)
	}
	out := make([]*flightv1.Flight, 0, len(flights))
	for _, flight := range flights {
		out = append(out, toProtoFlight(flight))
	}
	return &flightv1.SearchFlightsResponse{Flights: out}, nil
}

func (s *Server) GetFlight(ctx context.Context, req *flightv1.GetFlightRequest) (*flightv1.Flight, error) {
	if _, err := uuid.Parse(req.GetId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, "некорректный id рейса")
	}
	flight, err := s.flights.Get(ctx, req.GetId())
	if err != nil {
		return nil, statusError(err)
	}
	return toProtoFlight(flight), nil
}

func (s *Server) ReserveSeats(ctx context.Context, req *flightv1.ReserveSeatsRequest) (*flightv1.SeatReservation, error) {
	if _, err := uuid.Parse(req.GetFlightId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, "некорректный flight_id")
	}
	if _, err := uuid.Parse(req.GetBookingId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, "некорректный booking_id")
	}
	if req.GetSeatCount() < 1 {
		return nil, status.Error(codes.InvalidArgument, "количество мест должно быть положительным")
	}
	reservation, err := s.flights.Reserve(ctx, req.GetFlightId(), int(req.GetSeatCount()), req.GetBookingId())
	if err != nil {
		return nil, statusError(err)
	}
	return toProtoReservation(reservation), nil
}

func (s *Server) ReleaseReservation(ctx context.Context, req *flightv1.ReleaseReservationRequest) (*flightv1.SeatReservation, error) {
	if _, err := uuid.Parse(req.GetBookingId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, "некорректный booking_id")
	}
	reservation, err := s.flights.Release(ctx, req.GetBookingId())
	if err != nil {
		return nil, statusError(err)
	}
	return toProtoReservation(reservation), nil
}

func (s *Server) UpdateFlight(ctx context.Context, req *flightv1.UpdateFlightRequest) (*flightv1.Flight, error) {
	if _, err := uuid.Parse(req.GetId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, "некорректный id рейса")
	}
	price, err := decimal.NewFromString(req.GetPrice())
	if err != nil || !price.IsPositive() {
		return nil, status.Error(codes.InvalidArgument, "некорректная цена")
	}
	statusName, ok := flightStatusName(req.GetStatus())
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "неизвестный статус рейса")
	}
	if req.GetDepartureAt() == nil || req.GetArrivalAt() == nil {
		return nil, status.Error(codes.InvalidArgument, "нужны время вылета и прилёта")
	}
	departure := req.GetDepartureAt().AsTime()
	arrival := req.GetArrivalAt().AsTime()
	if !arrival.After(departure) {
		return nil, status.Error(codes.InvalidArgument, "прилёт должен быть позже вылета")
	}
	flight, err := s.flights.Update(ctx, req.GetId(), price, statusName, departure, arrival)
	if err != nil {
		return nil, statusError(err)
	}
	return toProtoFlight(flight), nil
}

func utcDay(ts *timestamppb.Timestamp) *time.Time {
	if ts == nil {
		return nil
	}
	value := ts.AsTime().UTC()
	day := time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
	return &day
}

func toProtoFlight(flight store.Flight) *flightv1.Flight {
	return &flightv1.Flight{
		Id:             flight.ID,
		FlightNumber:   flight.FlightNumber,
		Origin:         flight.Origin,
		Destination:    flight.Destination,
		DepartureAt:    timestamppb.New(flight.DepartureAt),
		ArrivalAt:      timestamppb.New(flight.ArrivalAt),
		TotalSeats:     int32(flight.TotalSeats),
		AvailableSeats: int32(flight.AvailableSeats),
		Price:          flight.Price.StringFixed(2),
		Status:         flightStatus(flight.Status),
	}
}

func toProtoReservation(reservation store.Reservation) *flightv1.SeatReservation {
	return &flightv1.SeatReservation{
		Id:        reservation.ID,
		FlightId:  reservation.FlightID,
		BookingId: reservation.BookingID,
		SeatCount: int32(reservation.SeatCount),
		Status:    reservationStatus(reservation.Status),
	}
}

func flightStatus(value string) flightv1.FlightStatus {
	switch value {
	case "SCHEDULED":
		return flightv1.FlightStatus_FLIGHT_STATUS_SCHEDULED
	case "DEPARTED":
		return flightv1.FlightStatus_FLIGHT_STATUS_DEPARTED
	case "CANCELLED":
		return flightv1.FlightStatus_FLIGHT_STATUS_CANCELLED
	case "COMPLETED":
		return flightv1.FlightStatus_FLIGHT_STATUS_COMPLETED
	default:
		return flightv1.FlightStatus_FLIGHT_STATUS_UNSPECIFIED
	}
}

func flightStatusName(value flightv1.FlightStatus) (string, bool) {
	switch value {
	case flightv1.FlightStatus_FLIGHT_STATUS_SCHEDULED:
		return "SCHEDULED", true
	case flightv1.FlightStatus_FLIGHT_STATUS_DEPARTED:
		return "DEPARTED", true
	case flightv1.FlightStatus_FLIGHT_STATUS_CANCELLED:
		return "CANCELLED", true
	case flightv1.FlightStatus_FLIGHT_STATUS_COMPLETED:
		return "COMPLETED", true
	default:
		return "", false
	}
}

func reservationStatus(value string) flightv1.ReservationStatus {
	switch value {
	case "ACTIVE":
		return flightv1.ReservationStatus_RESERVATION_STATUS_ACTIVE
	case "RELEASED":
		return flightv1.ReservationStatus_RESERVATION_STATUS_RELEASED
	case "EXPIRED":
		return flightv1.ReservationStatus_RESERVATION_STATUS_EXPIRED
	default:
		return flightv1.ReservationStatus_RESERVATION_STATUS_UNSPECIFIED
	}
}

func statusError(err error) error {
	var target *ferr.Error
	if errors.As(err, &target) {
		switch target.Kind {
		case ferr.KindInvalid:
			return status.Error(codes.InvalidArgument, target.Message)
		case ferr.KindNotFound:
			return status.Error(codes.NotFound, target.Message)
		case ferr.KindExhausted:
			return status.Error(codes.ResourceExhausted, target.Message)
		case ferr.KindPrecondition:
			return status.Error(codes.FailedPrecondition, target.Message)
		case ferr.KindAlreadyExists:
			return status.Error(codes.AlreadyExists, target.Message)
		}
	}
	return status.Error(codes.Internal, "внутренняя ошибка")
}
