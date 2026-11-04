package cache

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/shopspring/decimal"

	"flightbooking/internal/flight/rules"
	"flightbooking/internal/flight/store"
)

var ErrMiss = errors.New("cache miss")

type KV interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) error
}

type backend interface {
	Search(ctx context.Context, origin, destination string, day *time.Time) ([]store.Flight, error)
	Get(ctx context.Context, id string) (store.Flight, error)
	Reserve(ctx context.Context, flightID string, seatCount int, bookingID string) (store.Reservation, error)
	Release(ctx context.Context, bookingID string) (store.Reservation, error)
	Update(ctx context.Context, id string, price decimal.Decimal, status string, departure, arrival time.Time) (store.Flight, error)
}

type Cache struct {
	backend backend
	kv      KV
	ttl     time.Duration
	log     *slog.Logger
}

func New(backend backend, kv KV, ttl time.Duration, log *slog.Logger) *Cache {
	if log == nil {
		log = slog.Default()
	}
	return &Cache{backend: backend, kv: kv, ttl: ttl, log: log}
}

func (c *Cache) Search(ctx context.Context, origin, destination string, day *time.Time) ([]store.Flight, error) {
	key := searchKey(origin, destination, day)
	var flights []store.Flight
	if c.read(ctx, key, &flights) {
		if flights == nil {
			return []store.Flight{}, nil
		}
		return flights, nil
	}
	flights, err := c.backend.Search(ctx, origin, destination, day)
	if err != nil {
		return nil, err
	}
	c.write(ctx, key, flights)
	return flights, nil
}

func (c *Cache) Get(ctx context.Context, id string) (store.Flight, error) {
	key := flightKey(id)
	var flight store.Flight
	if c.read(ctx, key, &flight) {
		return flight, nil
	}
	flight, err := c.backend.Get(ctx, id)
	if err != nil {
		return store.Flight{}, err
	}
	c.write(ctx, key, flight)
	return flight, nil
}

func (c *Cache) Reserve(ctx context.Context, flightID string, seatCount int, bookingID string) (store.Reservation, error) {
	reservation, err := c.backend.Reserve(ctx, flightID, seatCount, bookingID)
	if err != nil {
		return store.Reservation{}, err
	}
	c.invalidate(ctx, reservation.FlightID)
	return reservation, nil
}

func (c *Cache) Release(ctx context.Context, bookingID string) (store.Reservation, error) {
	reservation, err := c.backend.Release(ctx, bookingID)
	if err != nil {
		return store.Reservation{}, err
	}
	c.invalidate(ctx, reservation.FlightID)
	return reservation, nil
}

func (c *Cache) Update(ctx context.Context, id string, price decimal.Decimal, status string, departure, arrival time.Time) (store.Flight, error) {
	flight, err := c.backend.Update(ctx, id, price, status, departure, arrival)
	if err != nil {
		return store.Flight{}, err
	}
	c.drop(ctx, flight)
	return flight, nil
}

func (c *Cache) read(ctx context.Context, key string, dest any) bool {
	payload, err := c.kv.Get(ctx, key)
	if err != nil {
		if !errors.Is(err, ErrMiss) {
			c.log.Error("cache get", "key", key, "err", err)
		}
		c.log.Info("cache", "result", "miss", "key", key)
		return false
	}
	if err := json.Unmarshal(payload, dest); err != nil {
		c.log.Error("cache decode", "key", key, "err", err)
		c.log.Info("cache", "result", "miss", "key", key)
		return false
	}
	c.log.Info("cache", "result", "hit", "key", key)
	return true
}

func (c *Cache) write(ctx context.Context, key string, value any) {
	payload, err := json.Marshal(value)
	if err != nil {
		c.log.Error("cache encode", "key", key, "err", err)
		return
	}
	if err := c.kv.Set(ctx, key, payload, c.ttl); err != nil {
		c.log.Error("cache set", "key", key, "err", err)
	}
}

func (c *Cache) invalidate(ctx context.Context, flightID string) {
	flight, err := c.backend.Get(ctx, flightID)
	if err != nil {
		c.log.Error("cache invalidate", "flight_id", flightID, "err", err)
		c.dropKeys(ctx, flightKey(flightID))
		return
	}
	c.drop(ctx, flight)
}

func (c *Cache) drop(ctx context.Context, flight store.Flight) {
	day := rules.DepartureDay(flight.DepartureAt)
	c.dropKeys(ctx,
		flightKey(flight.ID),
		searchKey(flight.Origin, flight.Destination, &day),
		searchKey(flight.Origin, flight.Destination, nil),
	)
}

func (c *Cache) dropKeys(ctx context.Context, keys ...string) {
	if err := c.kv.Del(ctx, keys...); err != nil {
		// Изменение в базе уже зафиксировано. Ошибку удаления только пишем в лог.
		c.log.Error("cache delete", "keys", keys, "err", err)
	}
}

func flightKey(id string) string {
	return "flight:" + id
}

func searchKey(origin, destination string, day *time.Time) string {
	if day == nil {
		return "search:" + origin + ":" + destination + ":all"
	}
	return "search:" + origin + ":" + destination + ":" + day.UTC().Format("2006-01-02")
}
