package cache

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"flightbooking/internal/flight/ferr"
	"flightbooking/internal/flight/store"
)

func TestCacheHitMissAndInvalidation(t *testing.T) {
	flight := sampleFlight()
	backend := &fakeBackend{flight: flight}
	kv := newMemKV()
	var logs bytes.Buffer
	c := New(backend, kv, time.Minute, slog.New(slog.NewJSONHandler(&logs, nil)))
	ctx := context.Background()
	day := flight.DepartureAt

	if _, err := c.Get(ctx, flight.ID); err != nil {
		t.Fatal(err)
	}
	got, err := c.Get(ctx, flight.ID)
	if err != nil {
		t.Fatal(err)
	}
	if backend.getCalls != 1 {
		t.Fatalf("повторное чтение сходило в базу %d раз", backend.getCalls)
	}
	if !got.Price.Equal(flight.Price) || got.AvailableSeats != flight.AvailableSeats {
		t.Fatalf("из кеша пришло %+v", got)
	}
	if kv.ttls[flightKey(flight.ID)] != time.Minute {
		t.Fatalf("ttl = %s", kv.ttls[flightKey(flight.ID)])
	}
	if _, err := c.Search(ctx, flight.Origin, flight.Destination, &day); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Search(ctx, flight.Origin, flight.Destination, &day); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Search(ctx, flight.Origin, flight.Destination, nil); err != nil {
		t.Fatal(err)
	}
	if backend.searchCalls != 2 {
		t.Fatalf("поиск сходил в базу %d раз", backend.searchCalls)
	}

	if _, err := c.Reserve(ctx, flight.ID, 1, "booking"); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		flightKey(flight.ID),
		searchKey(flight.Origin, flight.Destination, &day),
		searchKey(flight.Origin, flight.Destination, nil),
	} {
		if _, ok := kv.values[key]; ok {
			t.Fatalf("ключ %s остался после резерва", key)
		}
	}
	if _, err := c.Get(ctx, flight.ID); err != nil {
		t.Fatal(err)
	}
	if backend.getCalls != 3 {
		t.Fatalf("после инвалидации чтений базы %d", backend.getCalls)
	}

	text := logs.String()
	if !bytes.Contains(logs.Bytes(), []byte(`"result":"hit"`)) || !bytes.Contains(logs.Bytes(), []byte(`"result":"miss"`)) {
		t.Fatalf("лог кеша: %s", text)
	}
	if !bytes.Contains(logs.Bytes(), []byte(flightKey(flight.ID))) {
		t.Fatalf("в логе нет ключа: %s", text)
	}
}

func TestSearchCachesEmptyList(t *testing.T) {
	backend := &fakeBackend{emptySearch: true}
	c := New(backend, newMemKV(), time.Minute, slog.New(slog.DiscardHandler))
	day := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	for range 2 {
		flights, err := c.Search(context.Background(), "VKO", "LED", &day)
		if err != nil {
			t.Fatal(err)
		}
		if len(flights) != 0 {
			t.Fatalf("ожидался пустой список, пришло %d", len(flights))
		}
	}
	if backend.searchCalls != 1 {
		t.Fatalf("пустой поиск сходил в базу %d раз", backend.searchCalls)
	}
}

func TestGetDoesNotCacheMissingFlight(t *testing.T) {
	kv := newMemKV()
	backend := &fakeBackend{getErr: ferr.NotFound("рейс не найден")}
	c := New(backend, kv, time.Minute, slog.New(slog.DiscardHandler))
	_, err := c.Get(context.Background(), "missing")
	var target *ferr.Error
	if !errors.As(err, &target) || target.Kind != ferr.KindNotFound {
		t.Fatal(err)
	}
	if len(kv.values) != 0 {
		t.Fatal("отсутствие рейса попало в кеш")
	}
}

func TestCorruptEntryIsMiss(t *testing.T) {
	flight := sampleFlight()
	kv := newMemKV()
	kv.values[flightKey(flight.ID)] = []byte("not-json")
	backend := &fakeBackend{flight: flight}
	c := New(backend, kv, time.Minute, slog.New(slog.DiscardHandler))
	got, err := c.Get(context.Background(), flight.ID)
	if err != nil || got.FlightNumber != flight.FlightNumber {
		t.Fatal(err, got)
	}
	if backend.getCalls != 1 {
		t.Fatalf("битая запись не перечитана из базы")
	}
}

func TestRedisFailureDoesNotFailMutation(t *testing.T) {
	flight := sampleFlight()
	backend := &fakeBackend{flight: flight}
	c := New(backend, downKV{}, time.Minute, slog.New(slog.DiscardHandler))
	ctx := context.Background()
	got, err := c.Get(ctx, flight.ID)
	if err != nil || got.ID != flight.ID {
		t.Fatal(err)
	}
	if _, err := c.Reserve(ctx, flight.ID, 1, "booking"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Release(ctx, "booking"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Update(ctx, flight.ID, flight.Price, "DEPARTED", flight.DepartureAt, flight.ArrivalAt); err != nil {
		t.Fatal(err)
	}
	if backend.reserveCalls != 1 || backend.releaseCalls != 1 || backend.updateCalls != 1 {
		t.Fatalf("reserve=%d release=%d update=%d", backend.reserveCalls, backend.releaseCalls, backend.updateCalls)
	}
}

func TestUpdateDropsRouteKeys(t *testing.T) {
	flight := sampleFlight()
	backend := &fakeBackend{flight: flight}
	kv := newMemKV()
	c := New(backend, kv, time.Minute, slog.New(slog.DiscardHandler))
	ctx := context.Background()
	day := flight.DepartureAt
	if _, err := c.Get(ctx, flight.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Search(ctx, flight.Origin, flight.Destination, &day); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Search(ctx, flight.Origin, flight.Destination, nil); err != nil {
		t.Fatal(err)
	}
	updated := flight.DepartureAt.Add(15 * time.Minute)
	if _, err := c.Update(ctx, flight.ID, decimal.RequireFromString("8200.00"), "DEPARTED", updated, flight.ArrivalAt); err != nil {
		t.Fatal(err)
	}
	if _, ok := kv.values[flightKey(flight.ID)]; ok {
		t.Fatal("ключ рейса остался")
	}
	if _, ok := kv.values[searchKey(flight.Origin, flight.Destination, &day)]; ok {
		t.Fatal("ключ поиска на дату остался")
	}
	if _, ok := kv.values[searchKey(flight.Origin, flight.Destination, nil)]; ok {
		t.Fatal("ключ поиска без даты остался")
	}
}

type memKV struct {
	mu     sync.Mutex
	values map[string][]byte
	ttls   map[string]time.Duration
}

func newMemKV() *memKV {
	return &memKV{values: map[string][]byte{}, ttls: map[string]time.Duration{}}
}

func (m *memKV) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	value, ok := m.values[key]
	if !ok {
		return nil, ErrMiss
	}
	return append([]byte(nil), value...), nil
}

func (m *memKV) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.values[key] = append([]byte(nil), value...)
	m.ttls[key] = ttl
	return nil
}

func (m *memKV) Del(_ context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, key := range keys {
		delete(m.values, key)
		delete(m.ttls, key)
	}
	return nil
}

type downKV struct{}

func (downKV) Get(context.Context, string) ([]byte, error) {
	return nil, errors.New("redis down")
}

func (downKV) Set(context.Context, string, []byte, time.Duration) error {
	return errors.New("redis down")
}

func (downKV) Del(context.Context, ...string) error {
	return errors.New("redis down")
}

type fakeBackend struct {
	flight       store.Flight
	emptySearch  bool
	getErr       error
	getCalls     int
	searchCalls  int
	reserveCalls int
	releaseCalls int
	updateCalls  int
}

func (f *fakeBackend) Search(context.Context, string, string, *time.Time) ([]store.Flight, error) {
	f.searchCalls++
	if f.emptySearch {
		return []store.Flight{}, nil
	}
	return []store.Flight{f.flight}, nil
}

func (f *fakeBackend) Get(context.Context, string) (store.Flight, error) {
	f.getCalls++
	if f.getErr != nil {
		return store.Flight{}, f.getErr
	}
	return f.flight, nil
}

func (f *fakeBackend) Reserve(context.Context, string, int, string) (store.Reservation, error) {
	f.reserveCalls++
	return store.Reservation{FlightID: f.flight.ID, BookingID: "booking", SeatCount: 1, Status: "ACTIVE"}, nil
}

func (f *fakeBackend) Release(context.Context, string) (store.Reservation, error) {
	f.releaseCalls++
	return store.Reservation{FlightID: f.flight.ID, BookingID: "booking", SeatCount: 1, Status: "RELEASED"}, nil
}

func (f *fakeBackend) Update(_ context.Context, _ string, price decimal.Decimal, status string, departure, arrival time.Time) (store.Flight, error) {
	f.updateCalls++
	f.flight.Price = price
	f.flight.Status = status
	f.flight.DepartureAt = departure
	f.flight.ArrivalAt = arrival
	return f.flight, nil
}

func sampleFlight() store.Flight {
	return store.Flight{
		ID:             "00000000-0000-4000-8000-000000000001",
		FlightNumber:   "SU1001",
		Origin:         "VKO",
		Destination:    "LED",
		DepartureAt:    time.Date(2026, 4, 1, 7, 0, 0, 0, time.UTC),
		ArrivalAt:      time.Date(2026, 4, 1, 8, 30, 0, 0, time.UTC),
		TotalSeats:     5,
		AvailableSeats: 5,
		Price:          decimal.RequireFromString("4500.00"),
		Status:         "SCHEDULED",
	}
}
