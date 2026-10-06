// Package username centralizes username rules for user-service so
// handlers, services, and tests share one implementation.
package username

import (
	"errors"
	"strings"
	"unicode"
)

var (
	ErrInvalid  = errors.New("invalid username")
	ErrReserved = errors.New("username is reserved")
)

// reserved holds names that can never become profile URLs, route segments,
// or impersonate the platform. Kept in one place per platform policy.
var reserved = map[string]struct{}{
	"admin": {}, "administrator": {}, "moderator": {}, "system": {},
	"official": {}, "edvance": {},

	"api": {}, "auth": {}, "login": {}, "logout": {}, "register": {},
	"settings": {}, "support": {}, "help": {}, "about": {}, "contact": {},

	"courses": {}, "course": {}, "learn": {}, "learning": {},
	"creator": {}, "creators": {}, "studio": {}, "dashboard": {},
	"search": {}, "explore": {}, "notifications": {}, "messages": {},

	"user": {}, "users": {}, "me": {}, "profile": {},
}

// Normalize lowercases and trims a username.
func Normalize(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

// Validate enforces 3-30 chars of letters, digits, underscore, hyphen.
// Hyphen is accepted for compatibility with auth-service registration,
// which permits it; all other rules match the public profile-URL policy.
func Validate(username string) error {
	if len(username) < 3 || len(username) > 30 {
		return ErrInvalid
	}

	for _, char := range username {
		if unicode.IsLetter(char) ||
			unicode.IsDigit(char) ||
			char == '_' ||
			char == '-' {
			continue
		}

		return ErrInvalid
	}

	return nil
}

// IsReserved reports whether a (normalized) username is platform-reserved.
func IsReserved(username string) bool {
	_, ok := reserved[Normalize(username)]

	return ok
}

// Check validates and rejects reserved names, returning the specific
// reason for unavailable usernames.
func Check(username string) error {
	normalized := Normalize(username)

	if err := Validate(normalized); err != nil {
		return err
	}

	if IsReserved(normalized) {
		return ErrReserved
	}

	return nil
}
