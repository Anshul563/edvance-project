package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/notification-service/internal/model"
)

// PreferenceStore is the persistence contract for preferences.
// *repository.PreferenceRepository satisfies it.
type PreferenceStore interface {
	GetPreferences(ctx context.Context, userID uuid.UUID) (*model.Preference, error)
	UpsertPreferences(ctx context.Context, preference *model.Preference) error
}

// PreferenceService owns preference reads and updates. Reads return the
// defaults view when the user never customized anything. Updates coerce
// security_enabled to true: security notifications are mandatory and
// can never be disabled through this API.
type PreferenceService struct {
	preferences PreferenceStore
}

func NewPreferenceService(preferences PreferenceStore) (*PreferenceService, error) {
	if preferences == nil {
		return nil, errors.New("preference store is required")
	}

	return &PreferenceService{
		preferences: preferences,
	}, nil
}

// Get returns the effective preferences: stored row, or defaults.
func (s *PreferenceService) Get(
	ctx context.Context,
	userID uuid.UUID,
) (*model.Preference, error) {
	preference, err := s.preferences.GetPreferences(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load preferences: %w", err)
	}

	if preference == nil {
		return model.Defaults(userID), nil
	}

	return preference, nil
}

type UpdatePreferencesInput struct {
	EmailEnabled         *bool
	InAppEnabled         *bool
	MarketingEnabled     *bool
	CourseUpdatesEnabled *bool
	LearningEnabled      *bool
	PaymentEnabled       *bool
	SecurityEnabled      *bool
	CreatorEnabled       *bool
}

// Update applies a partial preference update, creating the row on first
// write. security_enabled is forced true regardless of input.
func (s *PreferenceService) Update(
	ctx context.Context,
	userID uuid.UUID,
	input UpdatePreferencesInput,
) (*model.Preference, error) {
	current, err := s.preferences.GetPreferences(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load preferences: %w", err)
	}

	if current == nil {
		current = model.Defaults(userID)
	}

	if input.EmailEnabled != nil {
		current.EmailEnabled = *input.EmailEnabled
	}

	if input.InAppEnabled != nil {
		current.InAppEnabled = *input.InAppEnabled
	}

	if input.MarketingEnabled != nil {
		current.MarketingEnabled = *input.MarketingEnabled
	}

	if input.CourseUpdatesEnabled != nil {
		current.CourseUpdatesEnabled = *input.CourseUpdatesEnabled
	}

	if input.LearningEnabled != nil {
		current.LearningEnabled = *input.LearningEnabled
	}

	if input.PaymentEnabled != nil {
		current.PaymentEnabled = *input.PaymentEnabled
	}

	// Mandatory: security notifications can never be disabled.
	current.SecurityEnabled = true

	if input.CreatorEnabled != nil {
		current.CreatorEnabled = *input.CreatorEnabled
	}

	current.UserID = userID

	if err := s.preferences.UpsertPreferences(ctx, current); err != nil {
		return nil, fmt.Errorf("save preferences: %w", err)
	}

	return current, nil
}
