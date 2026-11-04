-- flight_id без внешнего ключа.

CREATE TYPE booking_status AS ENUM ('CONFIRMED', 'CANCELLED');

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;

CREATE TABLE bookings (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    flight_id UUID NOT NULL,
    passenger_name VARCHAR(255) NOT NULL,
    passenger_email VARCHAR(255) NOT NULL,
    seat_count INTEGER NOT NULL,
    total_price NUMERIC(12, 2) NOT NULL,
    status booking_status NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT bookings_seats_ck CHECK (seat_count > 0),
    CONSTRAINT bookings_price_ck CHECK (total_price > 0),
    CONSTRAINT bookings_name_ck CHECK (char_length(passenger_name) > 0)
);

CREATE INDEX bookings_user_id_idx ON bookings (user_id);

CREATE TRIGGER bookings_set_updated_at
    BEFORE UPDATE ON bookings
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
