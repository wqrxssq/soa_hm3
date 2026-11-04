package store

import (
	"context"
	"time"

	"github.com/shopspring/decimal"

	"flightbooking/internal/flight/ferr"
	"flightbooking/internal/flight/rules"
	"flightbooking/internal/pgdb"
)

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

type Reservation struct {
	ID        string
	FlightID  string
	BookingID string
	SeatCount int
	Status    string
}

type Store struct {
	db *pgdb.Pool
}

func Connect(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgdb.Connect(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	return &Store{db: pool}, nil
}

func (s *Store) Close() {
	s.db.Close()
}

func (s *Store) Search(ctx context.Context, origin, destination string, day *time.Time) ([]Flight, error) {
	var dateArg any
	if day != nil {
		dateArg = *day
	}
	rows, err := s.db.Query().Query(ctx, `
		SELECT `+flightColumns+`
		FROM flights
		WHERE origin_iata = $1
		  AND destination_iata = $2
		  AND status = 'SCHEDULED'
		  AND ($3::date IS NULL OR departure_date = $3::date)
		ORDER BY departure_at, flight_number
	`, origin, destination, dateArg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Flight, 0)
	for rows.Next() {
		flight, err := scanFlight(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, flight)
	}
	return out, rows.Err()
}

func (s *Store) Get(ctx context.Context, id string) (Flight, error) {
	return scanFlight(s.db.Query().QueryRow(ctx, `
		SELECT `+flightColumns+`
		FROM flights
		WHERE id = $1::uuid
	`, id))
}

func (s *Store) Reserve(ctx context.Context, flightID string, seatCount int, bookingID string) (Reservation, error) {
	var out Reservation
	err := s.db.WithinTx(ctx, func(ctx context.Context, q pgdb.DBTX) error {
		// Блокировка строки рейса сериализует резерв последнего места.
		flight, err := scanFlight(q.QueryRow(ctx, `
			SELECT `+flightColumns+`
			FROM flights
			WHERE id = $1::uuid
			FOR UPDATE
		`, flightID))
		if err != nil {
			return err
		}
		if err := rules.EvaluateReserve(flight.Status, flight.AvailableSeats, seatCount); err != nil {
			return err
		}
		out, err = scanReservation(q.QueryRow(ctx, `
			INSERT INTO seat_reservations (flight_id, booking_id, seat_count, status)
			VALUES ($1::uuid, $2::uuid, $3, 'ACTIVE')
			RETURNING id::text, flight_id::text, booking_id::text, seat_count, status::text
		`, flightID, bookingID, seatCount))
		if pgdb.Unique(err) {
			return ferr.AlreadyExists("резервация уже существует")
		}
		if err != nil {
			return err
		}
		_, err = q.Exec(ctx, `
			UPDATE flights
			SET available_seats = available_seats - $2
			WHERE id = $1::uuid
		`, flightID, seatCount)
		return err
	})
	if err != nil {
		return Reservation{}, err
	}
	return out, nil
}

func (s *Store) Release(ctx context.Context, bookingID string) (Reservation, error) {
	var out Reservation
	err := s.db.WithinTx(ctx, func(ctx context.Context, q pgdb.DBTX) error {
		reservation, err := scanReservation(q.QueryRow(ctx, `
			SELECT id::text, flight_id::text, booking_id::text, seat_count, status::text
			FROM seat_reservations
			WHERE booking_id = $1::uuid
			FOR UPDATE
		`, bookingID))
		if err != nil {
			return err
		}
		action, err := rules.EvaluateRelease(reservation.Status)
		if err != nil {
			return err
		}
		if action == rules.Keep {
			out = reservation
			return nil
		}
		if err := q.QueryRow(ctx, `
			SELECT id::text
			FROM flights
			WHERE id = $1::uuid
			FOR UPDATE
		`, reservation.FlightID).Scan(&reservation.FlightID); err != nil {
			return err
		}
		if err := q.QueryRow(ctx, `
			UPDATE seat_reservations
			SET status = 'RELEASED'
			WHERE id = $1::uuid
			RETURNING id::text, flight_id::text, booking_id::text, seat_count, status::text
		`, reservation.ID).Scan(&out.ID, &out.FlightID, &out.BookingID, &out.SeatCount, &out.Status); err != nil {
			return err
		}
		_, err = q.Exec(ctx, `
			UPDATE flights
			SET available_seats = available_seats + $2
			WHERE id = $1::uuid
		`, out.FlightID, out.SeatCount)
		return err
	})
	if err != nil {
		return Reservation{}, err
	}
	return out, nil
}

func (s *Store) Update(ctx context.Context, id string, price decimal.Decimal, status string, departure, arrival time.Time) (Flight, error) {
	price = price.Round(2)
	current, err := s.Get(ctx, id)
	if err != nil {
		return Flight{}, err
	}
	if err := rules.ValidateUpdate(current.DepartureAt, departure, arrival, price, status); err != nil {
		return Flight{}, err
	}
	return scanFlight(s.db.Query().QueryRow(ctx, `
		UPDATE flights
		SET price = $2,
		    status = $3::flight_status,
		    departure_at = $4,
		    arrival_at = $5
		WHERE id = $1::uuid
		RETURNING `+flightColumns+`
	`, id, price.StringFixed(2), status, departure.UTC(), arrival.UTC()))
}

const flightColumns = `
	id::text, flight_number, TRIM(origin_iata), TRIM(destination_iata),
	departure_at, arrival_at, total_seats, available_seats, price::text, status::text
`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanFlight(row rowScanner) (Flight, error) {
	var flight Flight
	var price string
	err := row.Scan(
		&flight.ID,
		&flight.FlightNumber,
		&flight.Origin,
		&flight.Destination,
		&flight.DepartureAt,
		&flight.ArrivalAt,
		&flight.TotalSeats,
		&flight.AvailableSeats,
		&price,
		&flight.Status,
	)
	if pgdb.NoRows(err) {
		return Flight{}, ferr.NotFound("рейс не найден")
	}
	if err != nil {
		return Flight{}, err
	}
	flight.Price, err = decimal.NewFromString(price)
	if err != nil {
		return Flight{}, err
	}
	return flight, nil
}

func scanReservation(row rowScanner) (Reservation, error) {
	var reservation Reservation
	err := row.Scan(
		&reservation.ID,
		&reservation.FlightID,
		&reservation.BookingID,
		&reservation.SeatCount,
		&reservation.Status,
	)
	if pgdb.NoRows(err) {
		return Reservation{}, ferr.NotFound("резервация не найдена")
	}
	if err != nil {
		return Reservation{}, err
	}
	return reservation, nil
}
