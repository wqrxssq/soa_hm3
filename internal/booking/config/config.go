package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr           string
	DatabaseURL        string
	FlightGRPCAddr     string
	JWTSecret          string
	CBFailureThreshold int
	CBOpenTimeout      time.Duration
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
	threshold, openTimeout, err := circuitBreaker()
	if err != nil {
		return Config{}, err
	}
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	return Config{
		HTTPAddr:           addr,
		DatabaseURL:        databaseURL,
		FlightGRPCAddr:     flightAddr,
		JWTSecret:          secret,
		CBFailureThreshold: threshold,
		CBOpenTimeout:      openTimeout,
	}, nil
}

func circuitBreaker() (int, time.Duration, error) {
	threshold := 5
	if raw := os.Getenv("CB_FAILURE_THRESHOLD"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			return 0, 0, fmt.Errorf("CB_FAILURE_THRESHOLD must be a positive integer")
		}
		threshold = parsed
	}
	openTimeout := 15 * time.Second
	if raw := os.Getenv("CB_OPEN_TIMEOUT"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil || parsed <= 0 {
			return 0, 0, fmt.Errorf("CB_OPEN_TIMEOUT must be a positive duration")
		}
		openTimeout = parsed
	}
	return threshold, openTimeout, nil
}
