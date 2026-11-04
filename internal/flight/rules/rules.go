package rules

import (
	"time"

	"github.com/shopspring/decimal"

	"flightbooking/internal/flight/ferr"
)

const (
	Keep        = "keep"
	ReturnSeats = "return"
)

func EvaluateReserve(status string, available, requested int) error {
	if requested < 1 {
		return ferr.Invalid("количество мест должно быть положительным")
	}
	if status != "SCHEDULED" {
		return ferr.Precondition("рейс недоступен для бронирования")
	}
	if available < requested {
		return ferr.Exhausted("недостаточно мест")
	}
	return nil
}

func EvaluateRelease(status string) (string, error) {
	switch status {
	case "ACTIVE":
		return ReturnSeats, nil
	case "RELEASED":
		return Keep, nil
	case "EXPIRED":
		return "", ferr.Precondition("резервация просрочена")
	default:
		return "", ferr.Precondition("резервацию нельзя снять")
	}
}

func DepartureDay(value time.Time) time.Time {
	value = value.UTC()
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func ValidateUpdate(currentDeparture, departure, arrival time.Time, price decimal.Decimal, status string) error {
	if !price.IsPositive() {
		return ferr.Invalid("цена должна быть положительной")
	}
	switch status {
	case "SCHEDULED", "DEPARTED", "CANCELLED", "COMPLETED":
	default:
		return ferr.Invalid("неизвестный статус рейса")
	}
	if !arrival.After(departure) {
		return ferr.Invalid("прилёт должен быть позже вылета")
	}
	if !DepartureDay(currentDeparture).Equal(DepartureDay(departure)) {
		return ferr.Invalid("дата вылета должна совпадать с departure_date")
	}
	return nil
}
