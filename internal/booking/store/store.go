package store

import (
	"context"

	"flightbooking/internal/booksvc"
	"flightbooking/internal/pgdb"
)

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

func (s *Store) Insert(ctx context.Context, in booksvc.NewBooking) (booksvc.Booking, error) {
	return scanBooking(s.db.Query().QueryRow(ctx, `
		INSERT INTO bookings (
			id, user_id, flight_id, passenger_name, passenger_email,
			seat_count, total_price, status
		) VALUES (
			$1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7::numeric, 'CONFIRMED'
		)
		RETURNING `+bookingColumns+`
	`, in.ID, in.UserID, in.FlightID, in.PassengerName, in.PassengerEmail, in.SeatCount, in.TotalPrice.StringFixed(2)))
}

func (s *Store) Get(ctx context.Context, id string) (booksvc.Booking, error) {
	return scanBooking(s.db.Query().QueryRow(ctx, `
		SELECT `+bookingColumns+`
		FROM bookings
		WHERE id = $1::uuid
	`, id))
}

func (s *Store) ListByUser(ctx context.Context, userID string) ([]booksvc.Booking, error) {
	rows, err := s.db.Query().Query(ctx, `
		SELECT `+bookingColumns+`
		FROM bookings
		WHERE user_id = $1::uuid
		ORDER BY created_at, id
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]booksvc.Booking, 0)
	for rows.Next() {
		booking, err := scanBooking(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, booking)
	}
	return out, rows.Err()
}

func (s *Store) Cancel(ctx context.Context, id string) (booksvc.Booking, error) {
	return scanBooking(s.db.Query().QueryRow(ctx, `
		UPDATE bookings
		SET status = 'CANCELLED'
		WHERE id = $1::uuid AND status = 'CONFIRMED'
		RETURNING `+bookingColumns+`
	`, id))
}

const bookingColumns = `
	id::text, user_id::text, flight_id::text, passenger_name, passenger_email,
	seat_count, total_price::text, status::text, created_at, updated_at
`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanBooking(row rowScanner) (booksvc.Booking, error) {
	var booking booksvc.Booking
	var price string
	err := row.Scan(
		&booking.ID,
		&booking.UserID,
		&booking.FlightID,
		&booking.PassengerName,
		&booking.PassengerEmail,
		&booking.SeatCount,
		&price,
		&booking.Status,
		&booking.CreatedAt,
		&booking.UpdatedAt,
	)
	if pgdb.NoRows(err) {
		return booksvc.Booking{}, booksvc.ErrNotFound
	}
	if err != nil {
		return booksvc.Booking{}, err
	}
	booking.TotalPrice, err = decimalFrom(price)
	if err != nil {
		return booksvc.Booking{}, err
	}
	return booking, nil
}
