package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"flightbooking/internal/booking/config"
	"flightbooking/internal/booking/httpapi"
	"flightbooking/internal/booking/store"
	"flightbooking/internal/booksvc"
	"flightbooking/internal/flightclient"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	flights, err := flightclient.Dial(flightclient.DialConfig{
		Addr:               cfg.FlightGRPCAddr,
		Secret:             cfg.JWTSecret,
		CBFailureThreshold: cfg.CBFailureThreshold,
		CBOpenTimeout:      cfg.CBOpenTimeout,
		CallTimeout:        cfg.FlightCallTimeout,
		Logger:             slog.Default(),
	})
	if err != nil {
		log.Fatal(err)
	}
	defer flights.Close()

	handler, err := httpapi.New(booksvc.New(flights, st, slog.Default()))
	if err != nil {
		log.Fatal(err)
	}
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("booking listening on %s", cfg.HTTPAddr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
