package handle

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	if got := Normalize("  AnshulBuilds "); got != "anshulbuilds" {
		t.Fatalf("expected anshulbuilds, got %q", got)
	}
}

func TestValidHandles(t *testing.T) {
	for _, name := range []string{
		"anshulbuilds",
		"codewithanshul",
		"anshul_dev",
		"abc",
		strings.Repeat("a", 30),
		"dev-ops",
	} {
		if err := Check(name); err != nil {
			t.Fatalf("expected %q to be valid, got %v", name, err)
		}
	}
}

func TestInvalidHandles(t *testing.T) {
	for _, name := range []string{
		"",
		"a",
		"ab",
		strings.Repeat("a", 31),
		"@anshul",
		"anshul.dev",
		"anshul builds",
		"anshul!dev",
	} {
		if err := Check(name); !errors.Is(err, ErrInvalid) {
			t.Fatalf("expected %q to be invalid, got %v", name, err)
		}
	}
}

func TestReservedHandles(t *testing.T) {
	for _, name := range []string{
		"admin",
		"ADMIN",
		"api",
		"creator",
		"creators",
		"channel",
		"channels",
		"studio",
		"support",
		"settings",
		"edvance",
		"system",
	} {
		if err := Check(name); !errors.Is(err, ErrReserved) {
			t.Fatalf("expected %q to be reserved, got %v", name, err)
		}
	}
}
