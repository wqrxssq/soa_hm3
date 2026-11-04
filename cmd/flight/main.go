package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"flightbooking/internal/flight/cache"
	"flightbooking/internal/flight/config"
	"flightbooking/internal/flight/grpcserver"
	"flightbooking/internal/flight/store"
	flightv1 "flightbooking/internal/flightv1"
	"flightbooking/internal/svcjwt"
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

	kv := cache.NewSentinel(cfg.RedisMasterName, cfg.RedisSentinelAddrs)
	if err := kv.Ping(ctx); err != nil {
		log.Fatalf("redis: %v", err)
	}
	defer kv.Close()

	listener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		log.Fatal(err)
	}
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(svcjwt.UnaryServerInterceptor(cfg.JWTSecret)))
	flightv1.RegisterFlightServiceServer(grpcServer, grpcserver.New(cache.New(st, kv, cfg.CacheTTL, slog.Default())))
	go func() {
		log.Printf("flight grpc listening on %s", cfg.GRPCAddr)
		if err := grpcServer.Serve(listener); err != nil {
			log.Printf("flight grpc stopped: %v", err)
		}
	}()

	healthServer := &http.Server{
		Addr:              cfg.HealthAddr,
		Handler:           healthHandler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = healthServer.Shutdown(shutdownCtx)
		grpcServer.GracefulStop()
	}()

	log.Printf("flight health listening on %s", cfg.HealthAddr)
	if err := healthServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func healthHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"status\":\"ok\"}\n"))
	})
	return mux
}
