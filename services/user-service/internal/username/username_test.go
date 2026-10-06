package username

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	if got := Normalize("  Anshul_DEV "); got != "anshul_dev" {
		t.Fatalf("expected anshul_dev, got %q", got)
	}
}

func TestValidUsernames(t *testing.T) {
	for _, name := range []string{
		"anshul",
		"anshul563",
		"anshul_dev",
		"abc",
		strings.Repeat("a", 30),
		"user-name",
	} {
		if err := Check(name); err != nil {
			t.Fatalf("expected %q to be valid, got %v", name, err)
		}
	}
}

func TestInvalidUsernames(t *testing.T) {
	for _, name := range []string{
		"",
		"a",
		"ab",
		strings.Repeat("a", 31),
		"Anshul Shakya",
		"@anshul",
		"anshul.dev",
		"anshul!dev",
		"anshul/dev",
	} {
		if err := Check(name); !errors.Is(err, ErrInvalid) {
			t.Fatalf("expected %q to be invalid, got %v", name, err)
		}
	}
}

func TestReservedUsernames(t *testing.T) {
	for _, name := range []string{
		"admin",
		"ADMIN",
		"api",
		"login",
		"support",
		"courses",
		"creator",
		"dashboard",
		"users",
		"edvance",
		"system",
	} {
		if err := Check(name); !errors.Is(err, ErrReserved) {
			t.Fatalf("expected %q to be reserved, got %v", name, err)
		}
	}
}
