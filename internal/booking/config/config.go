package config

import (
	"fmt"
	"os"
)

type Config struct {
	HTTPAddr       string
	DatabaseURL    string
	FlightGRPCAddr string
	JWTSecret      string
}

func Load() (Config, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	flightAddr := os.Getenv("FLIGHT_GRPC_ADDR")
	if flightAddr == "" {
		return Config{}, fmt.Errorf("FLIGHT_GRPC_ADDR is required")
	}
	secret := os.Getenv("FLIGHT_JWT_SECRET")
	if secret == "" {
		return Config{}, fmt.Errorf("FLIGHT_JWT_SECRET is required")
	}
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	return Config{
		HTTPAddr:       addr,
		DatabaseURL:    databaseURL,
		FlightGRPCAddr: flightAddr,
		JWTSecret:      secret,
	}, nil
}
