# Flight booking

Два сервиса: Booking принимает REST, Flight хранит рейсы и отдаёт gRPC. У каждого своя PostgreSQL.

Стек: Go, OpenAPI, gRPC, PostgreSQL, Flyway.

В этом проходе есть транзакционный резерв мест. Аутентификация между сервисами, Redis, retry и circuit breaker появятся следующими проходами.

## Запуск

```bash
docker compose up --build
```

Booking слушает `http://localhost:8080`. Служебный health Flight — `http://localhost:8081/health`, gRPC — `localhost:9090`.

Базы: `localhost:5432` (`booking` / `booking`) и `localhost:5433` (`flight` / `flight`).

Проверка:

```bash
curl -sS http://localhost:8080/health
curl -sS 'http://localhost:8080/flights?origin=VKO&destination=LED&date=2026-04-01'
```

Остановка с удалением данных:

```bash
docker compose down -v
```

E2E ходит в уже поднятый compose и меняет остатки мест. Перед повторным прогоном нужен чистый volume:

```bash
docker compose down -v && docker compose up --build -d
python3 scripts/e2e.py
```

Кодогенерация (её же выполняет сборка образа):

```bash
make generate
```

`internal/flightv1/*.pb.go` и `internal/booking/api/openapi.gen.go` лежат в `.gitignore`.

## Схема данных

У рейса уникальна пара `(flight_number, departure_date)`. `flight_id` и `booking_id` связывают сервисы без внешнего ключа. Тот же текст лежит в [docs/er.mmd](docs/er.mmd).

```mermaid
erDiagram
    direction TB
    FLIGHTS ||--o{ SEAT_RESERVATIONS : flight_id
    FLIGHTS ||--o{ BOOKINGS : "flight_id, no FK"
    BOOKINGS ||--|| SEAT_RESERVATIONS : "booking_id, no FK"

    FLIGHTS {
        uuid id PK
        varchar flight_number
        char origin_iata
        char destination_iata
        date departure_date
        timestamptz departure_at
        timestamptz arrival_at
        int total_seats
        int available_seats
        numeric price
        enum status
        timestamptz created_at
        timestamptz updated_at
    }

    SEAT_RESERVATIONS {
        uuid id PK
        uuid flight_id FK
        uuid booking_id UK
        int seat_count
        enum status
        timestamptz created_at
        timestamptz updated_at
    }

    BOOKINGS {
        uuid id PK
        uuid user_id
        uuid flight_id
        varchar passenger_name
        varchar passenger_email
        int seat_count
        numeric total_price
        enum status
        timestamptz created_at
        timestamptz updated_at
    }
```

## Контракт

REST: `api/openapi/openapi.yaml`. gRPC: `api/proto/flight/v1/flight.proto`.

`user_id` клиент передаёт сам. Сервис не заводит пользователей и не проверяет владельца при чтении и отмене бронирования.

Демо-рейсы вставляет миграция Flight. `SU1001` (`00000000-0000-4000-8000-000000000001`) — пять мест VKO→LED на 2026-04-01.
