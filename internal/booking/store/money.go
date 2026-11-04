package store

import "github.com/shopspring/decimal"

func decimalFrom(value string) (decimal.Decimal, error) {
	return decimal.NewFromString(value)
}
