package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

func New(target string, stripPrefix string) (http.Handler, error) {
	targetURL, err := url.Parse(target)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	originalDirector := proxy.Director

	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		req.Header.Set("X-Edvance-Gateway", "api-gateway")

		if requestID := req.Header.Get("X-Request-ID"); requestID != "" {
			req.Header.Set("X-Request-ID", requestID)
		}

		req.URL.Path = strings.TrimPrefix(
			req.URL.Path,
			stripPrefix,
		)

		if req.URL.Path == "" {
			req.URL.Path = "/"
		}
	}

	proxy.ErrorHandler = func(
		w http.ResponseWriter,
		r *http.Request,
		err error,
	) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)

		_, _ = w.Write([]byte(
			`{"error":"service_unavailable"}`,
		))
	}

	return proxy, nil
}