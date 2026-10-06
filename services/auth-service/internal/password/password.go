// Package password centralizes password hashing and policy so
// registration, login, and password reset all share one implementation.
package password

import (
	"errors"
	"fmt"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

// ErrInvalidPassword is returned when a password violates the policy.
var ErrInvalidPassword = errors.New("invalid password")

// HashPassword hashes a password with bcrypt for storage.
// Plain passwords must never be stored.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		return "", fmt.Errorf("password: hash password: %w", err)
	}

	return string(hash), nil
}

// CheckPassword reports whether a plain password matches a stored hash.
func CheckPassword(hash string, password string) error {
	if err := bcrypt.CompareHashAndPassword(
		[]byte(hash),
		[]byte(password),
	); err != nil {
		return ErrInvalidPassword
	}

	return nil
}

// ValidatePassword enforces the Edvance password policy: 8-72 characters
// with at least one letter and one number.
func ValidatePassword(password string) error {
	if len(password) < 8 || len(password) > 72 {
		return ErrInvalidPassword
	}

	var hasLetter bool
	var hasNumber bool

	for _, char := range password {
		if unicode.IsLetter(char) {
			hasLetter = true
		}

		if unicode.IsDigit(char) {
			hasNumber = true
		}
	}

	if !hasLetter || !hasNumber {
		return ErrInvalidPassword
	}

	return nil
}
