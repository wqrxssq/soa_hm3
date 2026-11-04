package flightclient

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"flightbooking/internal/apperr"
	"flightbooking/internal/booksvc"
	flightv1 "flightbooking/internal/flightv1"
)

type Client struct {
	api  flightv1.FlightServiceClient
	conn *grpc.ClientConn
}

func Dial(addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &Client{api: flightv1.NewFlightServiceClient(conn), conn: conn}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Search(ctx context.Context, origin, destination string, day *time.Time) ([]booksvc.Flight, error) {
	req := &flightv1.SearchFlightsRequest{Origin: origin, Destination: destination}
	if day != nil {
		req.Date = timestamppb.New(*day)
	}
	resp, err := c.api.SearchFlights(ctx, req)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]booksvc.Flight, 0, len(resp.GetFlights()))
	for _, item := range resp.GetFlights() {
		flight, err := fromProto(item)
		if err != nil {
			return nil, err
		}
		out = append(out, flight)
	}
	return out, nil
}

func (c *Client) Get(ctx context.Context, id string) (booksvc.Flight, error) {
	resp, err := c.api.GetFlight(ctx, &flightv1.GetFlightRequest{Id: id})
	if err != nil {
		return booksvc.Flight{}, mapErr(err)
	}
	return fromProto(resp)
}

func (c *Client) Reserve(ctx context.Context, flightID string, seatCount int, bookingID string) error {
	_, err := c.api.ReserveSeats(ctx, &flightv1.ReserveSeatsRequest{
		FlightId:  flightID,
		SeatCount: int32(seatCount),
		BookingId: bookingID,
	})
	if err != nil {
		return mapErr(err)
	}
	return nil
}

func (c *Client) Release(ctx context.Context, bookingID string) error {
	_, err := c.api.ReleaseReservation(ctx, &flightv1.ReleaseReservationRequest{BookingId: bookingID})
	return mapErr(err)
}

func fromProto(flight *flightv1.Flight) (booksvc.Flight, error) {
	price, err := decimal.NewFromString(flight.GetPrice())
	if err != nil {
		return booksvc.Flight{}, apperr.New(500, "INTERNAL_ERROR", "внутренняя ошибка")
	}
	return booksvc.Flight{
		ID:             flight.GetId(),
		FlightNumber:   flight.GetFlightNumber(),
		Origin:         flight.GetOrigin(),
		Destination:    flight.GetDestination(),
		DepartureAt:    flight.GetDepartureAt().AsTime(),
		ArrivalAt:      flight.GetArrivalAt().AsTime(),
		TotalSeats:     int(flight.GetTotalSeats()),
		AvailableSeats: int(flight.GetAvailableSeats()),
		Price:          price,
		Status:         flightStatus(flight.GetStatus()),
	}, nil
}

func flightStatus(value flightv1.FlightStatus) string {
	switch value {
	case flightv1.FlightStatus_FLIGHT_STATUS_SCHEDULED:
		return "SCHEDULED"
	case flightv1.FlightStatus_FLIGHT_STATUS_DEPARTED:
		return "DEPARTED"
	case flightv1.FlightStatus_FLIGHT_STATUS_CANCELLED:
		return "CANCELLED"
	case flightv1.FlightStatus_FLIGHT_STATUS_COMPLETED:
		return "COMPLETED"
	default:
		return ""
	}
}

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return apperr.New(500, "INTERNAL_ERROR", "внутренняя ошибка")
	}
	switch st.Code() {
	case codes.InvalidArgument:
		return apperr.WithDetails(400, "VALIDATION_ERROR", "ошибка валидации", apperr.Fields("request", st.Message()))
	case codes.NotFound:
		return apperr.New(404, "NOT_FOUND", st.Message())
	case codes.ResourceExhausted:
		return apperr.New(409, "RESOURCE_EXHAUSTED", st.Message())
	case codes.FailedPrecondition:
		return apperr.New(409, "FAILED_PRECONDITION", st.Message())
	case codes.AlreadyExists:
		return apperr.New(409, "CONFLICT", st.Message())
	case codes.Unavailable:
		return apperr.New(503, "UNAVAILABLE", "сервис рейсов недоступен")
	case codes.DeadlineExceeded:
		return apperr.New(503, "UNAVAILABLE", "сервис рейсов недоступен")
	case codes.Unauthenticated:
		return apperr.New(500, "INTERNAL_ERROR", "внутренняя ошибка")
	default:
		return apperr.New(500, "INTERNAL_ERROR", "внутренняя ошибка")
	}
}
