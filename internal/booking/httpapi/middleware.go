package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"flightbooking/internal/apperr"
)

type contextKey int

const bagKey contextKey = iota

type requestBag struct {
	RequestID string
}

func accessLog(out io.Writer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			bag := &requestBag{}
			ctx := context.WithValue(r.Context(), bagKey, bag)
			recorder := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			started := time.Now()
			next.ServeHTTP(recorder, r.WithContext(ctx))
			line, err := json.Marshal(map[string]any{
				"request_id":  bag.RequestID,
				"method":      r.Method,
				"endpoint":    r.URL.RequestURI(),
				"status_code": recorder.status,
				"duration_ms": time.Since(started).Milliseconds(),
				"timestamp":   time.Now().UTC().Format(time.RFC3339Nano),
			})
			if err != nil {
				slog.Error("marshal access log", "err", err)
				return
			}
			_, _ = out.Write(append(line, '\n'))
		})
	}
}

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.NewString()
		if bag, ok := r.Context().Value(bagKey).(*requestBag); ok {
			bag.RequestID = id
		}
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r)
	})
}

func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("panic", "err", recovered)
				writeError(w, apperr.New(http.StatusInternalServerError, "INTERNAL_ERROR", "внутренняя ошибка"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (w *statusWriter) WriteHeader(status int) {
	if w.wrote {
		return
	}
	w.wrote = true
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(payload []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(payload)
}
