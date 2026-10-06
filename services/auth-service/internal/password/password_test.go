package password

import (
	"strings"
	"testing"
)

func TestValidatePassword(t *testing.T) {
	valid := []string{
		"Password123",
		"a1b2c3d4",
		"long-enough-1",
		"UPPER lower 9",
		"Pass 123",
	}

	for _, pw := range valid {
		if err := ValidatePassword(pw); err != nil {
			t.Fatalf("expected %q to be valid, got %v", pw, err)
		}
	}

	weak := []string{
		"",
		"short1",
		"password",
		"12345678",
		"!!!!!!!!",
		strings.Repeat("a1", 37),
	}

	for _, pw := range weak {
		if err := ValidatePassword(pw); err == nil {
			t.Fatalf("expected %q to be rejected", pw)
		}
	}
}

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("Password123")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	if hash == "" || hash == "Password123" {
		t.Fatal("expected a real hash, not the plain password")
	}

	if err := CheckPassword(hash, "Password123"); err != nil {
		t.Fatalf("check correct password: %v", err)
	}

	if err := CheckPassword(hash, "WrongPass1"); err == nil {
		t.Fatal("expected wrong password to fail")
	}
}

func TestHashPasswordSalting(t *testing.T) {
	first, err := HashPassword("Password123")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	second, err := HashPassword("Password123")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	if first == second {
		t.Fatal("expected unique salts per hash")
	}
}
