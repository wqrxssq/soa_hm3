package httpapi

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"flightbooking/internal/booking/api"
	"flightbooking/internal/booksvc"
)

func (s *Server) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, api.HealthResponse{Status: "ok"})
}

func (s *Server) SearchFlights(w http.ResponseWriter, r *http.Request, params api.SearchFlightsParams) {
	var day *time.Time
	if params.Date != nil {
		value := params.Date.Time
		utc := time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
		day = &utc
	}
	flights, err := s.books.Search(r.Context(), params.Origin, params.Destination, day)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]api.Flight, 0, len(flights))
	for _, flight := range flights {
		item, err := toFlight(flight)
		if err != nil {
			writeError(w, err)
			return
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) GetFlight(w http.ResponseWriter, r *http.Request, id api.ResourceId) {
	flight, err := s.books.GetFlight(r.Context(), uuid.UUID(id).String())
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := toFlight(flight)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) CreateBooking(w http.ResponseWriter, r *http.Request) {
	var body api.BookingCreate
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	booking, err := s.books.Create(r.Context(), booksvc.CreateInput{
		UserID:         uuid.UUID(body.UserId).String(),
		FlightID:       uuid.UUID(body.FlightId).String(),
		PassengerName:  body.PassengerName,
		PassengerEmail: string(body.PassengerEmail),
		SeatCount:      int(body.SeatCount),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := toBooking(booking)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) GetBooking(w http.ResponseWriter, r *http.Request, id api.ResourceId) {
	booking, err := s.books.GetBooking(r.Context(), uuid.UUID(id).String())
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := toBooking(booking)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) ListBookings(w http.ResponseWriter, r *http.Request, params api.ListBookingsParams) {
	bookings, err := s.books.List(r.Context(), uuid.UUID(params.UserId).String())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]api.Booking, 0, len(bookings))
	for _, booking := range bookings {
		item, err := toBooking(booking)
		if err != nil {
			writeError(w, err)
			return
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) CancelBooking(w http.ResponseWriter, r *http.Request, id api.ResourceId) {
	booking, err := s.books.Cancel(r.Context(), uuid.UUID(id).String())
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := toBooking(booking)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
