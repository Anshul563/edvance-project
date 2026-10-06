package middleware

import (
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

func Recoverer(next http.Handler) http.Handler {
	return middleware.Recoverer(next)
}
