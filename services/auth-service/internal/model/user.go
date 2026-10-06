package model

import (
	"time"

	"github.com/google/uuid"
)

type UserStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusSuspended UserStatus = "suspended"
	UserStatusDeleted   UserStatus = "deleted"
)

type User struct {
	ID            uuid.UUID
	Email         string
	Username      string
	PasswordHash  string
	DisplayName   string
	Status        UserStatus
	EmailVerified bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
