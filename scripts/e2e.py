#!/usr/bin/env python3
"""Сценарии Booking Service против уже поднятого docker compose."""

import json
import os
import subprocess
import time
from datetime import datetime
import urllib.error
import urllib.request
import uuid
from concurrent.futures import ThreadPoolExecutor

BASE = os.environ.get("BASE", "http://localhost:8080")
FLIGHT_HEALTH = os.environ.get("FLIGHT_HEALTH", "http://localhost:8081/health")
FLIGHT_GRPC_ADDR = os.environ.get("FLIGHT_GRPC_ADDR", "localhost:9090")
FLIGHT_JWT_SECRET = os.environ.get("FLIGHT_JWT_SECRET", "dev-flight-jwt-secret")
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

SU1001 = "00000000-0000-4000-8000-000000000001"
SU1002 = "00000000-0000-4000-8000-000000000002"
SU1003 = "00000000-0000-4000-8000-000000000003"
SU1004 = "00000000-0000-4000-8000-000000000004"
SU1005 = "00000000-0000-4000-8000-000000000005"


class ApiError(Exception):
    pass


def request(method, path, body=None, expected=200):
    data = None if body is None else json.dumps(body).encode()
    headers = {"Content-Type": "application/json"}
    req = urllib.request.Request(BASE + path, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=10) as response:
            raw = response.read()
            status = response.status
            payload = json.loads(raw) if raw else None
    except urllib.error.HTTPError as err:
        raw = err.read()
        status = err.code
        payload = json.loads(raw) if raw else None
    if status != expected:
        raise ApiError(f"{method} {path} -> {status}, want {expected}: {payload}")
    return payload


def request_status(method, path, body=None):
    data = None if body is None else json.dumps(body).encode()
    headers = {"Content-Type": "application/json"}
    req = urllib.request.Request(BASE + path, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=10) as response:
            raw = response.read()
            payload = json.loads(raw) if raw else None
            return response.status, payload
    except urllib.error.HTTPError as err:
        raw = err.read()
        payload = json.loads(raw) if raw else None
        return err.code, payload


def wait_health():
    deadline = time.time() + 90
    last = None
    while time.time() < deadline:
        try:
            booking = request("GET", "/health")
            with urllib.request.urlopen(FLIGHT_HEALTH, timeout=3) as response:
                flight = json.loads(response.read())
            if booking.get("status") == "ok" and flight.get("status") == "ok":
                return
        except Exception as err:  # noqa: BLE001 - ждём, пока compose поднимется
            last = err
        time.sleep(1)
    raise SystemExit(f"сервисы не поднялись: {last}")


def numbers(flights):
    return [item["flight_number"] for item in flights]


def redis_get(key):
    result = subprocess.run(
        ["docker", "compose", "exec", "-T", "redis", "redis-cli", "GET", key],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        raise SystemExit(result.stderr.strip() or "redis-cli failed")
    value = result.stdout.strip()
    if value in ("", "(nil)"):
        return None
    return value


def update_flight(flight_id, price, status, departure, arrival):
    return subprocess.run(
        [
            "go", "run", "./scripts/updateflight",
            "-addr", FLIGHT_GRPC_ADDR,
            "-secret", FLIGHT_JWT_SECRET,
            "-id", flight_id,
            "-price", price,
            "-status", status,
            "-departure", departure,
            "-arrival", arrival,
        ],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=False,
        timeout=120,
    )


def parse_time(value):
    return datetime.fromisoformat(value.replace("Z", "+00:00"))


def require_price(flight_id, price):
    body = request("GET", f"/flights/{flight_id}")
    if abs(body["price"] - price) > 0.001:
        raise SystemExit(f"цена {flight_id}: {body}")
    return body


def booking_body(user_id, flight_id, seat_count):
    return {
        "user_id": user_id,
        "flight_id": flight_id,
        "passenger_name": "Ada Lovelace",
        "passenger_email": f"{user_id[:8]}@example.com",
        "seat_count": seat_count,
    }


def main():
    wait_health()

    dated = request("GET", "/flights?origin=VKO&destination=LED&date=2026-04-01")
    dated_numbers = numbers(dated)
    if dated_numbers != ["SU1001"]:
        raise SystemExit(f"поиск на дату: {dated_numbers}")
    if redis_get("search:VKO:LED:2026-04-01") is None:
        raise SystemExit("поиск на дату не попал в кеш")

    undated = request("GET", "/flights?origin=VKO&destination=LED")
    undated_numbers = numbers(undated)
    if undated_numbers != ["SU1001", "SU1003"]:
        raise SystemExit(f"поиск без даты: {undated_numbers}")
    if redis_get("search:VKO:LED:all") is None:
        raise SystemExit("поиск без даты не попал в кеш")

    departed = request("GET", f"/flights/{SU1005}")
    if departed["flight_number"] != "SU1005" or departed["status"] != "DEPARTED":
        raise SystemExit(f"вылетевший рейс: {departed}")
    if redis_get(f"flight:{SU1005}") is None:
        raise SystemExit("рейс не попал в кеш")
    check_update_flight()

    missing = request("GET", f"/flights/{uuid.uuid4()}", expected=404)
    if missing["error_code"] != "NOT_FOUND":
        raise SystemExit(missing)

    invalid = request("GET", "/flights?destination=LED", expected=400)
    if invalid["error_code"] != "VALIDATION_ERROR":
        raise SystemExit(invalid)

    before_book = request("GET", f"/flights/{SU1002}")
    if before_book["available_seats"] != 100:
        raise SystemExit(f"остаток SU1002: {before_book}")
    if redis_get(f"flight:{SU1002}") is None:
        raise SystemExit("SU1002 не попал в кеш")

    buyer = str(uuid.uuid4())
    created = request(
        "POST",
        "/bookings",
        booking_body(buyer, SU1002, 2),
        expected=201,
    )
    if created["status"] != "CONFIRMED" or abs(created["total_price"] - 14400) > 0.001:
        raise SystemExit(f"создание: {created}")
    if created["user_id"] != buyer or created["seat_count"] != 2:
        raise SystemExit(f"поля брони: {created}")
    if redis_get(f"flight:{SU1002}") is not None:
        raise SystemExit("кеш рейса не сброшен после резерва")
    after_book = request("GET", f"/flights/{SU1002}")
    if after_book["available_seats"] != 98:
        raise SystemExit(f"остаток после брони: {after_book}")

    loaded = request("GET", f"/bookings/{created['id']}")
    if loaded["id"] != created["id"]:
        raise SystemExit(loaded)

    owned = request("GET", f"/bookings?user_id={buyer}")
    if [item["id"] for item in owned] != [created["id"]]:
        raise SystemExit(f"список: {owned}")

    other = request("GET", f"/bookings?user_id={uuid.uuid4()}")
    if other != []:
        raise SystemExit(f"чужой список: {other}")

    departed_try = request("POST", "/bookings", booking_body(str(uuid.uuid4()), SU1005, 1), expected=409)
    if departed_try["error_code"] != "FAILED_PRECONDITION":
        raise SystemExit(departed_try)

    cancelled_flight = request("POST", "/bookings", booking_body(str(uuid.uuid4()), SU1004, 1), expected=409)
    if cancelled_flight["error_code"] != "FAILED_PRECONDITION":
        raise SystemExit(cancelled_flight)

    unknown_flight = request("POST", "/bookings", booking_body(str(uuid.uuid4()), str(uuid.uuid4()), 1), expected=404)
    if unknown_flight["error_code"] != "NOT_FOUND":
        raise SystemExit(unknown_flight)

    overflow_user = str(uuid.uuid4())
    overflow = request("POST", "/bookings", booking_body(overflow_user, SU1001, 6), expected=409)
    if overflow["error_code"] != "RESOURCE_EXHAUSTED":
        raise SystemExit(overflow)
    if request("GET", f"/bookings?user_id={overflow_user}") != []:
        raise SystemExit("бронирование создано при нехватке мест")

    race_user = str(uuid.uuid4())

    def race_once(_):
        return request_status("POST", "/bookings", booking_body(race_user, SU1001, 1))

    with ThreadPoolExecutor(max_workers=8) as pool:
        results = list(pool.map(race_once, range(8)))
    successes = [(status, body) for status, body in results if status == 201]
    exhausted = [(status, body) for status, body in results if status == 409]
    other_results = [item for item in results if item[0] not in (201, 409)]
    if other_results or len(successes) != 5 or len(exhausted) != 3:
        raise SystemExit(f"гонка: success={len(successes)} exhausted={len(exhausted)} other={other_results}")
    if any(body.get("error_code") != "RESOURCE_EXHAUSTED" for _, body in exhausted):
        raise SystemExit(exhausted)

    after_race = request("GET", f"/flights/{SU1001}")
    if after_race["available_seats"] != 0:
        raise SystemExit(f"остаток после гонки: {after_race}")
    cached_empty = request("GET", "/flights?origin=VKO&destination=LED&date=2026-04-01")
    if len(cached_empty) != 1 or cached_empty[0]["available_seats"] != 0:
        raise SystemExit(f"поиск после гонки: {cached_empty}")

    race_list = request("GET", f"/bookings?user_id={race_user}")
    if len(race_list) != 5:
        raise SystemExit(f"список гонки: {len(race_list)}")

    cancelled = request("POST", f"/bookings/{successes[0][1]['id']}/cancel")
    if cancelled["status"] != "CANCELLED":
        raise SystemExit(cancelled)
    if redis_get(f"flight:{SU1001}") is not None or redis_get("search:VKO:LED:2026-04-01") is not None:
        raise SystemExit("кеш не сброшен после отмены")
    restored = request("GET", f"/flights/{SU1001}")
    if restored["available_seats"] != 1:
        raise SystemExit(f"остаток после отмены: {restored}")
    refreshed = request("GET", "/flights?origin=VKO&destination=LED&date=2026-04-01")
    if len(refreshed) != 1 or refreshed[0]["available_seats"] != 1:
        raise SystemExit(f"поиск после отмены: {refreshed}")

    again = request("POST", f"/bookings/{successes[0][1]['id']}/cancel", expected=409)
    if again["error_code"] != "CONFLICT":
        raise SystemExit(again)

    missing_booking = request("GET", f"/bookings/{uuid.uuid4()}", expected=404)
    if missing_booking["error_code"] != "NOT_FOUND":
        raise SystemExit(missing_booking)

    still_there = request("GET", f"/flights/{SU1003}")
    if still_there["available_seats"] != 100:
        raise SystemExit(still_there)

    print("e2e ok")


def check_update_flight():
    original_departure = "2026-04-01T09:00:00Z"
    original_arrival = "2026-04-01T13:00:00Z"
    try:
        rejected = update_flight(SU1005, "8100.00", "DEPARTED", "2026-04-02T09:00:00Z", "2026-04-02T13:00:00Z")
        if rejected.returncode == 0 or "InvalidArgument" not in rejected.stderr:
            raise SystemExit(f"сдвиг даты: {rejected.returncode} {rejected.stderr}")
        if redis_get(f"flight:{SU1005}") is None:
            raise SystemExit("кеш сброшен при отклонённом обновлении")

        changed = update_flight(SU1005, "8200.00", "DEPARTED", "2026-04-01T09:15:00Z", "2026-04-01T13:15:00Z")
        if changed.returncode != 0:
            raise SystemExit(changed.stderr)
        if redis_get(f"flight:{SU1005}") is not None:
            raise SystemExit("кеш не сброшен после UpdateFlight")
        body = require_price(SU1005, 8200)
        if body["status"] != "DEPARTED" or parse_time(body["departure_at"]) != parse_time("2026-04-01T09:15:00Z"):
            raise SystemExit(f"обновлённый рейс: {body}")
        if redis_get(f"flight:{SU1005}") is None:
            raise SystemExit("повторное чтение не заполнило кеш")
    finally:
        restored = update_flight(SU1005, "8100.00", "DEPARTED", original_departure, original_arrival)
        if restored.returncode != 0:
            raise SystemExit(f"не удалось вернуть SU1005: {restored.stderr}")
    require_price(SU1005, 8100)


if __name__ == "__main__":
    main()
