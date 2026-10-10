package server

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type Server struct{ httpServer *http.Server }

func New(addr string, handler http.Handler) *Server {
	return &Server{httpServer: &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}}
}

func (s *Server) Start() error { return s.httpServer.ListenAndServe() }

func (s *Server) Shutdown(ctx context.Context) error { return s.httpServer.Shutdown(ctx) }

func (s *Server) Addr() string { return fmt.Sprintf("http://%s", s.httpServer.Addr) }
