package config

import "testing"

func TestLoadRequiresAllowedAdminUserIDs(t *testing.T) {
	if _, err := Load(); err == nil {
		t.Fatal("expected Load to fail when no admin users are configured")
	}
}
