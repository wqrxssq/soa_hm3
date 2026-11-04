package httpapi

import (
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"

	"flightbooking/internal/apperr"
	"flightbooking/internal/booking/api"
	"flightbooking/internal/booksvc"
)

type Server struct {
	books *booksvc.Service
}

func New(books *booksvc.Service) (http.Handler, error) {
	spec, err := api.GetSwagger()
	if err != nil {
		return nil, err
	}
	validator, err := newValidator(spec)
	if err != nil {
		return nil, err
	}
	server := &Server{books: books}
	router := chi.NewRouter()
	router.Use(accessLog(os.Stdout))
	router.Use(recoverer)
	router.Use(requestID)
	router.Use(validator)
	router.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, routeNotFound())
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, methodNotAllowed())
	})
	return api.HandlerWithOptions(server, api.ChiServerOptions{
		BaseRouter: router,
		ErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
			writeError(w, apperr.WithDetails(http.StatusBadRequest, "VALIDATION_ERROR", "ошибка валидации", apperr.Fields("request", err.Error())))
		},
	}), nil
}
