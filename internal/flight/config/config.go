package config

import (
	"fmt"
	"os"
)

type Config struct {
	GRPCAddr    string
	HealthAddr  string
	DatabaseURL string
}

func Load() (Config, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	grpcAddr := os.Getenv("GRPC_ADDR")
	if grpcAddr == "" {
		grpcAddr = ":9090"
	}
	healthAddr := os.Getenv("HEALTH_ADDR")
	if healthAddr == "" {
		healthAddr = ":8081"
	}
	return Config{
		GRPCAddr:    grpcAddr,
		HealthAddr:  healthAddr,
		DatabaseURL: databaseURL,
	}, nil
}
