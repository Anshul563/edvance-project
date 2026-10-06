package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/Anshul563/edvance-project/services/auth-service/internal/config"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/email"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/router"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/server"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/service"
)

func main() {
	logger := slog.New(
		slog.NewJSONHandler(os.Stdout, nil),
	)

	slog.SetDefault(logger)

	// Development convenience: load .env from the service directory.
	// Real environment variables always take precedence, so this is a
	// no-op in production where .env does not exist.
	if err := godotenv.Load(); err != nil {
		slog.Debug(
			"no .env file loaded, using process environment",
		)
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error(
			"failed to load config",
			"error",
			err,
		)
		os.Exit(1)
	}

	startupCtx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	db, err := repository.NewPostgres(
		startupCtx,
		cfg.Database.URL,
	)
	if err != nil {
		slog.Error(
			"failed to connect to postgres",
			"error",
			err,
		)
		os.Exit(1)
	}
	defer db.Close()

	redisClient, err := repository.NewRedis(
		startupCtx,
		cfg.Redis.URL,
	)
	if err != nil {
		slog.Error(
			"failed to connect to redis",
			"error",
			err,
		)
		os.Exit(1)
	}
	defer redisClient.Close()

	userRepository := repository.NewUserRepository(db)
	sessionRepository := repository.NewSessionRepository(db)

	sessionService, err := service.NewSessionService(
		sessionRepository,
		service.SessionConfig{
			AccessSecret: cfg.Auth.JWTAccessSecret,
			Issuer:       cfg.Auth.JWTIssuer,
			Audience:     cfg.Auth.JWTAudience,
			AccessTTL:    cfg.Auth.AccessTokenTTL,
			RefreshTTL:   cfg.Auth.RefreshTokenTTL,
		},
	)
	if err != nil {
		slog.Error(
			"failed to create session service",
			"error",
			err,
		)
		os.Exit(1)
	}

	emailSender, err := newEmailSender(cfg)
	if err != nil {
		slog.Error(
			"failed to create email sender",
			"error",
			err,
		)
		os.Exit(1)
	}

	verificationService, err := service.NewEmailVerificationService(
		repository.NewEmailVerificationRepository(db),
		userRepository,
		emailSender,
		repository.NewResendLimiter(
			redisClient,
			cfg.Email.ResendCooldown,
		),
		service.EmailVerificationConfig{
			TokenTTL:            cfg.Email.TokenTTL,
			VerificationBaseURL: cfg.Email.VerificationURL,
		},
	)
	if err != nil {
		slog.Error(
			"failed to create verification service",
			"error",
			err,
		)
		os.Exit(1)
	}

	authService := service.NewAuthService(
		userRepository,
		sessionService,
		cfg.Auth.AccessTokenTTL,
		verificationService,
	)

	resetService, err := service.NewPasswordResetService(
		repository.NewPasswordResetRepository(db),
		userRepository,
		emailSender,
		repository.NewPasswordResetLimiter(
			redisClient,
			cfg.Reset.Cooldown,
		),
		repository.NewPasswordResetIPLimiter(
			redisClient,
			cfg.Reset.IPCooldown,
		),
		service.PasswordResetConfig{
			TokenTTL:     cfg.Reset.TokenTTL,
			ResetBaseURL: cfg.Reset.BaseURL,
		},
	)
	if err != nil {
		slog.Error(
			"failed to create password reset service",
			"error",
			err,
		)
		os.Exit(1)
	}

	healthHandler := handler.NewHealthHandler(
		db,
		redisClient,
	)

	srv := server.New(
		cfg,
		router.Handlers{
			Health:   healthHandler,
			Register: handler.NewRegisterHandler(authService),
			Login:    handler.NewLoginHandler(authService),
			Refresh:  handler.NewRefreshHandler(authService),
			Logout:   handler.NewLogoutHandler(authService),
			Session:  handler.NewSessionHandler(authService),
			Verify:   handler.NewVerifyEmailHandler(verificationService),
			Resend:   handler.NewResendVerificationHandler(verificationService),
			Forgot:   handler.NewForgotPasswordHandler(resetService),
			Reset:    handler.NewResetPasswordHandler(resetService),
		},
	)

	serverErr := make(chan error, 1)

	go func() {
		slog.Info(
			"auth service starting",
			"port",
			cfg.Port,
			"environment",
			cfg.AppEnv,
		)

		serverErr <- srv.Start()
	}()

	sig := make(chan os.Signal, 1)

	signal.Notify(
		sig,
		syscall.SIGINT,
		syscall.SIGTERM,
	)

	select {
	case err := <-serverErr:
		if !errors.Is(err, os.ErrClosed) {
			slog.Error(
				"server stopped",
				"error",
				err,
			)
			os.Exit(1)
		}

	case signal := <-sig:
		slog.Info(
			"shutdown signal received",
			"signal",
			signal,
		)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error(
			"graceful shutdown failed",
			"error",
			err,
		)
		os.Exit(1)
	}

	slog.Info("auth service stopped")
}

// newEmailSender builds the configured email provider. Config validation
// already guarantees console mode never runs in production.
func newEmailSender(cfg config.Config) (email.Sender, error) {
	switch cfg.Email.Provider {
	case "smtp":
		return email.NewSMTPSender(email.SMTPConfig{
			Host:     cfg.Email.SMTPHost,
			Port:     cfg.Email.SMTPPort,
			Username: cfg.Email.SMTPUsername,
			Password: cfg.Email.SMTPPassword,
			From:     cfg.Email.From,
		})

	default:
		return email.NewConsoleSender(cfg.Email.From)
	}
}
