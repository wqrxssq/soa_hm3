package config

import (
	"testing"
	"time"
)

func TestLoadRequiresSecret(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("FLIGHT_GRPC_ADDR", "flight:9090")
	t.Setenv("FLIGHT_JWT_SECRET", "")
	t.Setenv("CB_FAILURE_THRESHOLD", "")
	t.Setenv("CB_OPEN_TIMEOUT", "")
	if _, err := Load(); err == nil {
		t.Fatal("пустой секрет принят")
	}
	t.Setenv("FLIGHT_JWT_SECRET", "secret")
	t.Setenv("HTTP_ADDR", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":8080" || cfg.JWTSecret != "secret" || cfg.CBFailureThreshold != 5 || cfg.CBOpenTimeout != 15*time.Second {
		t.Fatalf("%+v", cfg)
	}
}

func TestLoadCircuitBreaker(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("FLIGHT_GRPC_ADDR", "flight:9090")
	t.Setenv("FLIGHT_JWT_SECRET", "secret")
	t.Setenv("CB_FAILURE_THRESHOLD", "4")
	t.Setenv("CB_OPEN_TIMEOUT", "20s")
	cfg, err := Load()
	if err != nil || cfg.CBFailureThreshold != 4 || cfg.CBOpenTimeout != 20*time.Second {
		t.Fatalf("%+v %v", cfg, err)
	}
	t.Setenv("CB_FAILURE_THRESHOLD", "0")
	if _, err := Load(); err == nil {
		t.Fatal("нулевой порог принят")
	}
	t.Setenv("CB_FAILURE_THRESHOLD", "5")
	t.Setenv("CB_OPEN_TIMEOUT", "0s")
	if _, err := Load(); err == nil {
		t.Fatal("нулевой таймаут принят")
	}
}
