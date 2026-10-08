package config

import (
	"os"
	"testing"
)

func TestWorkerDefaults(t *testing.T) {
	t.Setenv("JWT_ACCESS_SECRET", "secret")
	t.Setenv("NOTIFICATION_INTERNAL_TOKEN", "token")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if cfg.Worker.Count != 4 {
		t.Fatalf("expected 4 workers, got %d", cfg.Worker.Count)
	}

	if cfg.Worker.MaxAttempts != 4 {
		t.Fatalf("expected 4 attempts, got %d", cfg.Worker.MaxAttempts)
	}
}

func TestWorkerValidation(t *testing.T) {
	t.Setenv("JWT_ACCESS_SECRET", "secret")
	t.Setenv("NOTIFICATION_INTERNAL_TOKEN", "token")

	for _, key := range []string{
		"NOTIFICATION_WORKER_COUNT",
		"NOTIFICATION_RETRY_INTERVAL",
		"NOTIFICATION_MAX_ATTEMPTS",
	} {
		os.Unsetenv(key)
	}

	t.Setenv("NOTIFICATION_WORKER_COUNT", "0")

	if _, err := Load(); err == nil {
		t.Fatal("expected worker count validation")
	}

	os.Unsetenv("NOTIFICATION_WORKER_COUNT")
	t.Setenv("NOTIFICATION_RETRY_INTERVAL", "bogus")

	if _, err := Load(); err == nil {
		t.Fatal("expected interval validation")
	}

	os.Unsetenv("NOTIFICATION_RETRY_INTERVAL")
	t.Setenv("NOTIFICATION_MAX_ATTEMPTS", "0")

	if _, err := Load(); err == nil {
		t.Fatal("expected attempts validation")
	}
}

func TestConsoleForbiddenInProduction(t *testing.T) {
	t.Setenv("JWT_ACCESS_SECRET", "secret")
	t.Setenv("NOTIFICATION_INTERNAL_TOKEN", "token")
	t.Setenv("APP_ENV", "production")
	t.Setenv("EMAIL_PROVIDER", "console")

	if _, err := Load(); err == nil {
		t.Fatal("expected console-in-production rejection")
	}
}
