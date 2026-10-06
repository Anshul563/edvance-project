package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Anshul563/edvance-project/services/video-service/internal/config"
	"github.com/Anshul563/edvance-project/services/video-service/internal/router"
)

type Server struct {
	httpServer *http.Server
}

func New(
	cfg config.Config,
	handlers router.Handlers,
) *Server {
	httpServer := &http.Server{
		Addr: fmt.Sprintf(":%d", cfg.Port),

		Handler: router.New(
			handlers,
			router.NewMiddleware(
				cfg.JWT.AccessSecret,
				cfg.JWT.Issuer,
				cfg.JWT.Audience,
				cfg.Internal.APIKey,
			),
		),

		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return &Server{
		httpServer: httpServer,
	}
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
