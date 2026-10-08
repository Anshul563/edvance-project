package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/content-service/internal/model"
)

// maxSlugAttempts bounds the duplicate-slug retry loop. The database
// UNIQUE constraint guarantees uniqueness regardless; the loop only
// makes the common (non-racing) case land on the base slug.
const maxSlugAttempts = 10

// maxCounterDelta bounds a single internal counter adjustment so a
// buggy consumer cannot warp a counter into absurdity.
const maxCounterDelta = 1_000_000

// validateTitle trims and bounds a title.
func validateTitle(raw string, limits Limits) (string, error) {
	title := strings.TrimSpace(raw)
	length := utf8.RuneCountInString(title)

	if length < 3 || length > limits.MaxTitleLength {
		return "", fmt.Errorf(
			"%w: title must be between 3 and %d characters",
			ErrInvalidInput,
			limits.MaxTitleLength,
		)
	}

	return title, nil
}

// validateDescription bounds an optional description. Empty input
// becomes nil (NULL), never an empty string.
func validateDescription(
	raw *string,
	limits Limits,
) (*string, error) {
	if raw == nil {
		return nil, nil
	}

	value := strings.TrimSpace(*raw)

	if value == "" {
		return nil, nil
	}

	if utf8.RuneCountInString(value) > limits.MaxDescriptionLength {
		return nil, fmt.Errorf(
			"%w: description must be at most %d characters",
			ErrInvalidInput,
			limits.MaxDescriptionLength,
		)
	}

	return &value, nil
}

// validateTags bounds tag counts; each tag is normalized later by the
// repository, but unusable input is rejected here so clients get a 400
// instead of silently losing tags.
func validateTags(tags []string, limits Limits) error {
	if len(tags) > limits.MaxTagsPerItem {
		return fmt.Errorf(
			"%w: at most %d tags are allowed",
			ErrInvalidInput,
			limits.MaxTagsPerItem,
		)
	}

	for _, raw := range tags {
		if _, _, ok := model.NormalizeTag(raw); !ok {
			return fmt.Errorf(
				"%w: tag %q has no usable characters",
				ErrInvalidInput,
				raw,
			)
		}
	}

	return nil
}

// parseVideoVisibility accepts empty (defaults to private) or a valid
// enum value.
func parseVideoVisibility(raw string) (model.VideoVisibility, error) {
	value := strings.TrimSpace(raw)

	if value == "" {
		return model.VideoVisibilityPrivate, nil
	}

	visibility := model.VideoVisibility(value)
	if !visibility.Valid() {
		return "", fmt.Errorf(
			"%w: visibility must be private, unlisted or public",
			ErrInvalidVisibility,
		)
	}

	return visibility, nil
}

// parseShortVisibility mirrors parseVideoVisibility for shorts.
func parseShortVisibility(raw string) (model.ShortVisibility, error) {
	value := strings.TrimSpace(raw)

	if value == "" {
		return model.ShortVisibilityPrivate, nil
	}

	visibility := model.ShortVisibility(value)
	if !visibility.Valid() {
		return "", fmt.Errorf(
			"%w: visibility must be private, unlisted or public",
			ErrInvalidVisibility,
		)
	}

	return visibility, nil
}

// parsePostVisibility mirrors parseVideoVisibility for posts.
func parsePostVisibility(raw string) (model.PostVisibility, error) {
	value := strings.TrimSpace(raw)

	if value == "" {
		return model.PostVisibilityPrivate, nil
	}

	visibility := model.PostVisibility(value)
	if !visibility.Valid() {
		return "", fmt.Errorf(
			"%w: visibility must be private or public",
			ErrInvalidVisibility,
		)
	}

	return visibility, nil
}

// normalizeOptionalText trims an optional URL-ish field, treats blank
// input as unset, and enforces a length ceiling.
func normalizeOptionalText(
	raw *string,
	maxRunes int,
) (*string, error) {
	if raw == nil {
		return nil, nil
	}

	value := strings.TrimSpace(*raw)

	if value == "" {
		return nil, nil
	}

	if utf8.RuneCountInString(value) > maxRunes {
		return nil, fmt.Errorf(
			"%w: value must be at most %d characters",
			ErrInvalidInput,
			maxRunes,
		)
	}

	return &value, nil
}

// validateDeltas bounds internal counter adjustments.
func validateDeltas(deltas ...int64) error {
	for _, delta := range deltas {
		if delta > maxCounterDelta || delta < -maxCounterDelta {
			return fmt.Errorf(
				"%w: counter delta %d exceeds +/-%d",
				ErrInvalidInput,
				delta,
				maxCounterDelta,
			)
		}
	}

	return nil
}

// viewerOrNil returns the viewer's creator ID for list scoping, or nil
// when the actor is anonymous or cannot be resolved.
func viewerOrNil(
	ctx context.Context,
	creators CreatorResolver,
	actor Actor,
) *uuid.UUID {
	id := viewerCreatorID(ctx, creators, actor)
	if id == uuid.Nil {
		return nil
	}

	return &id
}
