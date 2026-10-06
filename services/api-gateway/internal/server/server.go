package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Anshul563/edvance-project/services/api-gateway/internal/config"
	"github.com/Anshul563/edvance-project/services/api-gateway/internal/router"
)

type Server struct {
	httpServer *http.Server
}

func New(cfg config.Config) (*Server, error) {
	handler, err := router.New(cfg)
	if err != nil {
		return nil, err
	}

	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return &Server{
		httpServer: httpServer,
	}, nil
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
