package rules

import "flightbooking/internal/flight/ferr"

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
