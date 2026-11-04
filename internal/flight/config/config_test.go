package config

import (
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("FLIGHT_JWT_SECRET", "secret")
	t.Setenv("REDIS_ADDR", "redis:6379")
	t.Setenv("CACHE_TTL", "")
	t.Setenv("GRPC_ADDR", "")
	t.Setenv("HEALTH_ADDR", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CacheTTL != 5*time.Minute || cfg.GRPCAddr != ":9090" || cfg.HealthAddr != ":8081" || cfg.JWTSecret != "secret" {
		t.Fatalf("%+v", cfg)
	}
}

func TestLoadRequiresSecretAndRedis(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("REDIS_ADDR", "redis:6379")
	t.Setenv("FLIGHT_JWT_SECRET", "")
	if _, err := Load(); err == nil {
		t.Fatal("пустой секрет принят")
	}
	t.Setenv("FLIGHT_JWT_SECRET", "secret")
	t.Setenv("REDIS_ADDR", "")
	if _, err := Load(); err == nil {
		t.Fatal("пустой REDIS_ADDR принят")
	}
}

func TestLoadCacheTTL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want time.Duration
		ok   bool
	}{
		{name: "default", raw: "", want: 5 * time.Minute, ok: true},
		{name: "min", raw: "5m", want: 5 * time.Minute, ok: true},
		{name: "max", raw: "10m", want: 10 * time.Minute, ok: true},
		{name: "inside", raw: "6m30s", want: 6*time.Minute + 30*time.Second, ok: true},
		{name: "below", raw: "4m59s", ok: false},
		{name: "above", raw: "10m1s", ok: false},
		{name: "junk", raw: "nope", ok: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://example")
			t.Setenv("FLIGHT_JWT_SECRET", "secret")
			t.Setenv("REDIS_ADDR", "redis:6379")
			t.Setenv("CACHE_TTL", test.raw)
			cfg, err := Load()
			if test.ok {
				if err != nil || cfg.CacheTTL != test.want {
					t.Fatalf("got %s, err %v", cfg.CacheTTL, err)
				}
				return
			}
			if err == nil {
				t.Fatal("ожидалась ошибка")
			}
		})
	}
}
