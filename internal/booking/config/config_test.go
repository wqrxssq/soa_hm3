package config

import "testing"

func TestLoadRequiresSecret(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("FLIGHT_GRPC_ADDR", "flight:9090")
	t.Setenv("FLIGHT_JWT_SECRET", "")
	if _, err := Load(); err == nil {
		t.Fatal("пустой секрет принят")
	}
	t.Setenv("FLIGHT_JWT_SECRET", "secret")
	t.Setenv("HTTP_ADDR", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":8080" || cfg.JWTSecret != "secret" {
		t.Fatalf("%+v", cfg)
	}
}
