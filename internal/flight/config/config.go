package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	GRPCAddr           string
	HealthAddr         string
	DatabaseURL        string
	RedisSentinelAddrs []string
	RedisMasterName    string
	CacheTTL           time.Duration
	JWTSecret          string
}

func Load() (Config, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	secret := os.Getenv("FLIGHT_JWT_SECRET")
	if secret == "" {
		return Config{}, fmt.Errorf("FLIGHT_JWT_SECRET is required")
	}
	sentinels, masterName, err := redisSentinel()
	if err != nil {
		return Config{}, err
	}
	ttl, err := cacheTTL()
	if err != nil {
		return Config{}, err
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
		GRPCAddr:           grpcAddr,
		HealthAddr:         healthAddr,
		DatabaseURL:        databaseURL,
		RedisSentinelAddrs: sentinels,
		RedisMasterName:    masterName,
		CacheTTL:           ttl,
		JWTSecret:          secret,
	}, nil
}

func redisSentinel() ([]string, string, error) {
	var addrs []string
	for _, part := range strings.Split(os.Getenv("REDIS_SENTINEL_ADDR"), ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			addrs = append(addrs, part)
		}
	}
	if len(addrs) == 0 {
		return nil, "", fmt.Errorf("REDIS_SENTINEL_ADDR is required")
	}
	name := os.Getenv("REDIS_MASTER_NAME")
	if name == "" {
		name = "flight-master"
	}
	return addrs, name, nil
}

func cacheTTL() (time.Duration, error) {
	raw := os.Getenv("CACHE_TTL")
	if raw == "" {
		return 5 * time.Minute, nil
	}
	ttl, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("CACHE_TTL: %w", err)
	}
	if ttl < 5*time.Minute || ttl > 10*time.Minute {
		return 0, fmt.Errorf("CACHE_TTL must be from 5m to 10m")
	}
	return ttl, nil
}
