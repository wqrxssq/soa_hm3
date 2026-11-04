-- Дата вылета выводится из departure_at, отдельный столбец нарушал 3NF.

ALTER TABLE flights
    DROP CONSTRAINT flights_number_date_uq,
    DROP CONSTRAINT flights_date_ck;

DROP INDEX flights_search_idx;

ALTER TABLE flights DROP COLUMN departure_date;

CREATE UNIQUE INDEX flights_number_day_uq
    ON flights (flight_number, ((departure_at AT TIME ZONE 'UTC')::date));

CREATE INDEX flights_search_idx
    ON flights (origin_iata, destination_iata, ((departure_at AT TIME ZONE 'UTC')::date));
