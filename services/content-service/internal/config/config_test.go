package config

import "testing"

// loadWith resets the knobs this suite cares about so a developer's
// shell (or CI environment) cannot change the expectations.
func loadWith(t *testing.T) {
	t.Helper()

	t.Setenv("CONTENT_SERVICE_PORT", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("JWT_ACCESS_SECRET", "")
	t.Setenv("JWT_ISSUER", "")
	t.Setenv("JWT_AUDIENCE", "")
	t.Setenv("CREATOR_SERVICE_URL", "")
	t.Setenv("NOTIFICATION_SERVICE_URL", "")
	t.Setenv("NOTIFICATION_INTERNAL_TOKEN", "")
	t.Setenv("INTERNAL_API_KEY", "")
	t.Setenv("MAX_SHORT_DURATION_SECONDS", "")
	t.Setenv("DEFAULT_PAGE_SIZE", "")
	t.Setenv("MAX_PAGE_SIZE", "")
	t.Setenv("MAX_TAGS_PER_ITEM", "")
}

func TestLoadDefaults(t *testing.T) {
	loadWith(t)
	t.Setenv("JWT_ACCESS_SECRET", "dev-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if cfg.Port != 8084 {
		t.Fatalf("expected port 8084, got %d", cfg.Port)
	}

	if cfg.JWT.Issuer != "edvance-auth" || cfg.JWT.Audience != "edvance-api" {
		t.Fatalf("unexpected JWT config: %+v", cfg.JWT)
	}

	if cfg.Database.URL == "" {
		t.Fatal("database URL must have a default")
	}

	if cfg.Content.MaxShortDurationSeconds != 180 ||
		cfg.Content.DefaultPageSize != 20 ||
		cfg.Content.MaxPageSize != 100 ||
		cfg.Content.MaxTagsPerItem != 10 {
		t.Fatalf("unexpected content limits: %+v", cfg.Content)
	}

	if cfg.Internal.APIKey != "" {
		t.Fatal("internal key must default to empty (fail closed)")
	}
}

func TestLoadRequiresJWTSecret(t *testing.T) {
	loadWith(t)

	if _, err := Load(); err == nil {
		t.Fatal("missing JWT_ACCESS_SECRET must fail")
	}
}

func TestLoadRejectsInvalidNumbers(t *testing.T) {
	cases := map[string]string{
		"CONTENT_SERVICE_PORT":       "not-a-port",
		"MAX_SHORT_DURATION_SECONDS": "abc",
		"DEFAULT_PAGE_SIZE":          "0",
		"MAX_PAGE_SIZE":              "1",
		"MAX_TAGS_PER_ITEM":          "-1",
	}

	for key, value := range cases {
		t.Run(key, func(t *testing.T) {
			loadWith(t)
			t.Setenv("JWT_ACCESS_SECRET", "dev-secret")
			t.Setenv(key, value)

			if _, err := Load(); err == nil {
				t.Fatalf("%s=%s must fail", key, value)
			}
		})
	}
}

func TestLoadAcceptsExplicitSettings(t *testing.T) {
	loadWith(t)

	t.Setenv("JWT_ACCESS_SECRET", "dev-secret")
	t.Setenv("CONTENT_SERVICE_PORT", "9000")
	t.Setenv("INTERNAL_API_KEY", "shared-key")
	t.Setenv("CREATOR_SERVICE_URL", "http://creator:8083")
	t.Setenv("MAX_SHORT_DURATION_SECONDS", "60")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if cfg.Port != 9000 {
		t.Fatalf("expected port 9000, got %d", cfg.Port)
	}

	if cfg.Internal.APIKey != "shared-key" {
		t.Fatalf("expected shared-key, got %q", cfg.Internal.APIKey)
	}

	if cfg.CreatorServiceURL != "http://creator:8083" {
		t.Fatalf("unexpected creator URL: %q", cfg.CreatorServiceURL)
	}

	if cfg.Content.MaxShortDurationSeconds != 60 {
		t.Fatalf("expected 60, got %d", cfg.Content.MaxShortDurationSeconds)
	}
}
