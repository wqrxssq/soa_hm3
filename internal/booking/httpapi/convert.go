package httpapi

import (
	"net/http"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/shopspring/decimal"

	"flightbooking/internal/apperr"
	"flightbooking/internal/booking/api"
	"flightbooking/internal/booksvc"
)

func moneyFloat(value decimal.Decimal) float64 {
	asFloat, _ := value.Round(2).Float64()
	return asFloat
}

func parseID(value string) (api.ResourceId, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return api.ResourceId{}, apperr.New(http.StatusInternalServerError, "INTERNAL_ERROR", "внутренняя ошибка")
	}
	return api.ResourceId(id), nil
}

func toFlight(flight booksvc.Flight) (api.Flight, error) {
	id, err := parseID(flight.ID)
	if err != nil {
		return api.Flight{}, err
	}
	return api.Flight{
		Id:             id,
		FlightNumber:   flight.FlightNumber,
		Origin:         flight.Origin,
		Destination:    flight.Destination,
		DepartureAt:    flight.DepartureAt,
		ArrivalAt:      flight.ArrivalAt,
		TotalSeats:     flight.TotalSeats,
		AvailableSeats: flight.AvailableSeats,
		Price:          moneyFloat(flight.Price),
		Status:         api.FlightStatus(flight.Status),
	}, nil
}

func toBooking(booking booksvc.Booking) (api.Booking, error) {
	id, err := parseID(booking.ID)
	if err != nil {
		return api.Booking{}, err
	}
	userID, err := parseID(booking.UserID)
	if err != nil {
		return api.Booking{}, err
	}
	flightID, err := parseID(booking.FlightID)
	if err != nil {
		return api.Booking{}, err
	}
	return api.Booking{
		Id:             id,
		UserId:         userID,
		FlightId:       flightID,
		PassengerName:  booking.PassengerName,
		PassengerEmail: openapi_types.Email(booking.PassengerEmail),
		SeatCount:      booking.SeatCount,
		TotalPrice:     moneyFloat(booking.TotalPrice),
		Status:         api.BookingStatus(booking.Status),
		CreatedAt:      booking.CreatedAt,
		UpdatedAt:      booking.UpdatedAt,
	}, nil
}
