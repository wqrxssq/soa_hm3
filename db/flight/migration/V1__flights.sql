CREATE TYPE flight_status AS ENUM ('SCHEDULED', 'DEPARTED', 'CANCELLED', 'COMPLETED');
CREATE TYPE reservation_status AS ENUM ('ACTIVE', 'RELEASED', 'EXPIRED');

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;

CREATE TABLE flights (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    flight_number VARCHAR(16) NOT NULL,
    origin_iata CHAR(3) NOT NULL,
    destination_iata CHAR(3) NOT NULL,
    departure_date DATE NOT NULL,
    departure_at TIMESTAMPTZ NOT NULL,
    arrival_at TIMESTAMPTZ NOT NULL,
    total_seats INTEGER NOT NULL,
    available_seats INTEGER NOT NULL,
    price NUMERIC(12, 2) NOT NULL,
    status flight_status NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT flights_number_date_uq UNIQUE (flight_number, departure_date),
    CONSTRAINT flights_route_ck CHECK (origin_iata <> destination_iata),
    CONSTRAINT flights_iata_ck CHECK (
        origin_iata ~ '^[A-Z]{3}$' AND destination_iata ~ '^[A-Z]{3}$'
    ),
    CONSTRAINT flights_number_ck CHECK (flight_number ~ '^[A-Z0-9]{2,16}$'),
    CONSTRAINT flights_time_ck CHECK (arrival_at > departure_at),
    CONSTRAINT flights_date_ck CHECK ((departure_at AT TIME ZONE 'UTC')::date = departure_date),
    CONSTRAINT flights_seats_ck CHECK (
        total_seats > 0
        AND available_seats >= 0
        AND available_seats <= total_seats
    ),
    CONSTRAINT flights_price_ck CHECK (price > 0)
);

CREATE INDEX flights_search_idx
    ON flights (origin_iata, destination_iata, departure_date);

CREATE TRIGGER flights_set_updated_at
    BEFORE UPDATE ON flights
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE seat_reservations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    flight_id UUID NOT NULL REFERENCES flights (id),
    booking_id UUID NOT NULL,
    seat_count INTEGER NOT NULL,
    status reservation_status NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT seat_reservations_booking_uq UNIQUE (booking_id),
    CONSTRAINT seat_reservations_seats_ck CHECK (seat_count > 0)
);

CREATE INDEX seat_reservations_flight_id_idx ON seat_reservations (flight_id);

CREATE TRIGGER seat_reservations_set_updated_at
    BEFORE UPDATE ON seat_reservations
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
