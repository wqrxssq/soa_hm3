-- Рейсы для e2e.

INSERT INTO flights (
    id,
    flight_number,
    origin_iata,
    destination_iata,
    departure_date,
    departure_at,
    arrival_at,
    total_seats,
    available_seats,
    price,
    status
) VALUES
    (
        '00000000-0000-4000-8000-000000000001',
        'SU1001', 'VKO', 'LED',
        DATE '2026-04-01',
        TIMESTAMPTZ '2026-04-01 07:00:00+00',
        TIMESTAMPTZ '2026-04-01 08:30:00+00',
        5, 5, 4500.00, 'SCHEDULED'
    ),
    (
        '00000000-0000-4000-8000-000000000002',
        'SU1002', 'LED', 'VKO',
        DATE '2026-04-01',
        TIMESTAMPTZ '2026-04-01 12:00:00+00',
        TIMESTAMPTZ '2026-04-01 13:30:00+00',
        100, 100, 7200.00, 'SCHEDULED'
    ),
    (
        '00000000-0000-4000-8000-000000000003',
        'SU1003', 'VKO', 'LED',
        DATE '2026-04-02',
        TIMESTAMPTZ '2026-04-02 07:00:00+00',
        TIMESTAMPTZ '2026-04-02 08:30:00+00',
        100, 100, 4500.00, 'SCHEDULED'
    ),
    (
        '00000000-0000-4000-8000-000000000004',
        'SU1004', 'VKO', 'LED',
        DATE '2026-04-01',
        TIMESTAMPTZ '2026-04-01 18:00:00+00',
        TIMESTAMPTZ '2026-04-01 19:30:00+00',
        100, 100, 4500.00, 'CANCELLED'
    ),
    (
        '00000000-0000-4000-8000-000000000005',
        'SU1005', 'SVO', 'AER',
        DATE '2026-04-01',
        TIMESTAMPTZ '2026-04-01 09:00:00+00',
        TIMESTAMPTZ '2026-04-01 13:00:00+00',
        100, 100, 8100.00, 'DEPARTED'
    );
