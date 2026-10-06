// Package handle centralizes creator-handle rules so handlers, services,
// and tests share one implementation. Rules mirror the username policy:
// 3-30 chars, lowercase, letters/digits/underscore/hyphen.
package handle

import (
	"errors"
	"strings"
	"unicode"
)

var (
	ErrInvalid  = errors.New("invalid handle")
	ErrReserved = errors.New("handle is reserved")
)

// reserved holds names that can never become channel slugs, route
// segments, or impersonate the platform. Kept in one place.
var reserved = map[string]struct{}{
	"admin": {}, "administrator": {}, "moderator": {}, "system": {},
	"official": {}, "edvance": {},

	"api": {}, "auth": {}, "login": {}, "logout": {}, "register": {},
	"support": {}, "help": {}, "settings": {}, "dashboard": {},
	"search": {}, "explore": {},

	"creator": {}, "creators": {}, "channel": {}, "channels": {},
	"studio": {}, "user": {}, "users": {}, "me": {},

	"courses": {}, "course": {}, "learn": {}, "videos": {},
}

// Normalize lowercases and trims a handle.
func Normalize(handle string) string {
	return strings.ToLower(strings.TrimSpace(handle))
}

// Validate enforces 3-30 chars of letters, digits, underscore, hyphen.
func Validate(handle string) error {
	if len(handle) < 3 || len(handle) > 30 {
		return ErrInvalid
	}

	for _, char := range handle {
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

// IsReserved reports whether a (normalized) handle is platform-reserved.
func IsReserved(handle string) bool {
	_, ok := reserved[Normalize(handle)]

	return ok
}

// Check validates and rejects reserved handles, returning the specific
// reason for unavailable handles.
func Check(handle string) error {
	normalized := Normalize(handle)

	if err := Validate(normalized); err != nil {
		return err
	}

	if IsReserved(normalized) {
		return ErrReserved
	}

	return nil
}
