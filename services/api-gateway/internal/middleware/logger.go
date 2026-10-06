package middleware

import (
	"net/http"

	"github.com/go-chi/httplog/v2"
)

func Logger(next http.Handler) http.Handler {
	return httplog.RequestLogger(httplog.NewLogger("edvance-api-gateway", httplog.Options{
		JSON: true,
	}))(next)
}
