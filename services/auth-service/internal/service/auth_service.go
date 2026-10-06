package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode"

	"golang.org/x/crypto/bcrypt"

	"github.com/Anshul563/edvance-project/services/auth-service/internal/model"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/repository"
)

var (
	ErrInvalidEmail       = errors.New("invalid email")
	ErrInvalidUsername    = errors.New("invalid username")
	ErrInvalidPassword    = errors.New("invalid password")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrUsernameExists     = errors.New("username already exists")
)

type AuthService struct {
	userRepository UserStore
	sessions       *SessionService
	accessTTL      time.Duration
}

func NewAuthService(
	userRepository UserStore,
	sessionService *SessionService,
	accessTTL time.Duration,
) *AuthService {
	return &AuthService{
		userRepository: userRepository,
		sessions:       sessionService,
		accessTTL:      accessTTL,
	}
}

type RegisterInput struct {
	Email       string
	Username    string
	Password    string
	DisplayName string
}

type RegisterOutput struct {
	User *model.User
}

func (s *AuthService) Register(
	ctx context.Context,
	input RegisterInput,
) (*RegisterOutput, error) {

	email := normalizeEmail(input.Email)
	username := normalizeUsername(input.Username)
	displayName := strings.TrimSpace(input.DisplayName)

	if err := validateEmail(email); err != nil {
		return nil, err
	}

	if err := validateUsername(username); err != nil {
		return nil, err
	}

	if err := validatePassword(input.Password); err != nil {
		return nil, err
	}

	if displayName == "" {
		return nil, errors.New("display name is required")
	}

	_, err := s.userRepository.FindByEmail(ctx, email)

	if err == nil {
		return nil, ErrEmailAlreadyExists
	}

	if !errors.Is(err, repository.ErrUserNotFound) {
		return nil, fmt.Errorf("check email: %w", err)
	}

	_, err = s.userRepository.FindByUsername(ctx, username)

	if err == nil {
		return nil, ErrUsernameExists
	}

	if !errors.Is(err, repository.ErrUserNotFound) {
		return nil, fmt.Errorf("check username: %w", err)
	}

	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte(input.Password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &model.User{
		Email:         email,
		Username:      username,
		PasswordHash:  string(passwordHash),
		DisplayName:   displayName,
		Status:        model.UserStatusActive,
		EmailVerified: false,
	}

	if err := s.userRepository.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	return &RegisterOutput{
		User: user,
	}, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func normalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func validateEmail(email string) error {
	if email == "" || len(email) > 320 {
		return ErrInvalidEmail
	}

	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email {
		return ErrInvalidEmail
	}

	return nil
}

func validateUsername(username string) error {
	if len(username) < 3 || len(username) > 30 {
		return ErrInvalidUsername
	}

	for _, char := range username {
		if unicode.IsLetter(char) ||
			unicode.IsDigit(char) ||
			char == '_' ||
			char == '-' {
			continue
		}

		return ErrInvalidUsername
	}

	return nil
}

func validatePassword(password string) error {
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
