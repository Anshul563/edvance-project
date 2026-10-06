package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/creator-service/internal/handle"
	"github.com/Anshul563/edvance-project/services/creator-service/internal/model"
	"github.com/Anshul563/edvance-project/services/creator-service/internal/repository"
)

var (
	ErrCreatorAlreadyExists = errors.New("creator already exists")
	ErrCreatorNotFound      = errors.New("creator not found")
	ErrChannelNotFound      = errors.New("channel not found")
	ErrHandleTaken          = errors.New("handle already taken")
	ErrInvalidHandle        = errors.New("invalid handle")
	ErrReservedHandle       = errors.New("handle is reserved")
	ErrCreatorDisabled      = errors.New("creator is disabled")
	ErrCreatorSuspended     = errors.New("creator is suspended")
)

// CreatorStore is the persistence contract for creators.
// *repository.CreatorRepository satisfies it.
type CreatorStore interface {
	Onboard(ctx context.Context, creator *model.Creator, channel *model.Channel) error
	FindByUserID(ctx context.Context, userID uuid.UUID) (*model.Creator, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.Creator, error)
	UpdateCreator(ctx context.Context, creator *model.Creator) error
	CreatorExists(ctx context.Context, userID uuid.UUID) (bool, error)
}

// ChannelStore is the persistence contract for channels.
// *repository.ChannelRepository satisfies it.
type ChannelStore interface {
	FindChannelByCreatorID(ctx context.Context, creatorID uuid.UUID) (*model.Channel, error)
	FindByHandle(ctx context.Context, handle string) (*model.Channel, error)
	UpdateChannel(ctx context.Context, channel *model.Channel) error
	HandleExists(ctx context.Context, handle string) (bool, error)
}

// Future ownership (documented per platform boundaries):
//
//	creator-service WILL own: creator, channel, creator settings,
//	    creator status, creator metadata.
//	creator-service will NEVER own: courses, videos, followers,
//	    payments, earnings, analytics (future dedicated services).
type CreatorService struct {
	creators CreatorStore
	channels ChannelStore
}

func NewCreatorService(
	creators CreatorStore,
	channels ChannelStore,
) *CreatorService {
	return &CreatorService{
		creators: creators,
		channels: channels,
	}
}

type OnboardInput struct {
	ChannelName string
	Handle      string
	Description string
}

type CreatorProfile struct {
	Creator *model.Creator
	Channel *model.Channel
}

// OnboardCreator creates a creator and its channel atomically for the
// authenticated user. V1 decision: onboarding succeeds straight into
// `active` — no admin verification workflow yet; status transitions
// (suspend/disable) remain for future moderation flows.
//
// Identity comes only from the JWT (userID). No user-existence network
// call is made: the JWT signature itself is the proof of identity, and
// user-service owns profile data this service must not duplicate.
func (s *CreatorService) OnboardCreator(
	ctx context.Context,
	userID uuid.UUID,
	input OnboardInput,
) (*CreatorProfile, error) {
	channelName := strings.TrimSpace(input.ChannelName)

	if channelName == "" || utf8.RuneCountInString(channelName) > 100 {
		return nil, errors.New("channel name must be 1-100 characters")
	}

	normalized := handle.Normalize(input.Handle)

	if err := handle.Check(normalized); err != nil {
		if errors.Is(err, handle.ErrReserved) {
			return nil, ErrReservedHandle
		}

		return nil, ErrInvalidHandle
	}

	description := strings.TrimSpace(input.Description)

	if utf8.RuneCountInString(description) > 1000 {
		return nil, errors.New("description must be at most 1000 characters")
	}

	exists, err := s.creators.CreatorExists(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("check creator: %w", err)
	}

	if exists {
		return nil, ErrCreatorAlreadyExists
	}

	taken, err := s.channels.HandleExists(ctx, normalized)
	if err != nil {
		return nil, fmt.Errorf("check handle: %w", err)
	}

	if taken {
		return nil, ErrHandleTaken
	}

	creator := &model.Creator{
		UserID:      userID,
		Status:      model.CreatorStatusActive,
		DisplayName: channelName,
	}

	channel := &model.Channel{
		Handle:      normalized,
		Name:        channelName,
		Description: nullable(description),
		Status:      "active",
	}

	if err := s.creators.Onboard(ctx, creator, channel); err != nil {
		switch {
		case errors.Is(err, repository.ErrCreatorExists):
			return nil, ErrCreatorAlreadyExists

		case errors.Is(err, repository.ErrHandleTaken):
			return nil, ErrHandleTaken

		default:
			return nil, fmt.Errorf("onboard creator: %w", err)
		}
	}

	return &CreatorProfile{
		Creator: creator,
		Channel: channel,
	}, nil
}

// GetMyCreator returns the caller's creator and channel, regardless of
// status — owners can always see their own standing.
func (s *CreatorService) GetMyCreator(
	ctx context.Context,
	userID uuid.UUID,
) (*CreatorProfile, error) {
	creator, err := s.creators.FindByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrCreatorNotFound) {
			return nil, ErrCreatorNotFound
		}

		return nil, fmt.Errorf("find creator: %w", err)
	}

	channel, err := s.channels.FindChannelByCreatorID(ctx, creator.ID)
	if err != nil {
		return nil, fmt.Errorf("find channel: %w", err)
	}

	return &CreatorProfile{
		Creator: creator,
		Channel: channel,
	}, nil
}

// GetCreatorByHandle returns the public creator profile. Non-active
// creators (suspended/disabled) read as not found: public URLs must not
// leak moderation state.
func (s *CreatorService) GetCreatorByHandle(
	ctx context.Context,
	rawHandle string,
) (*CreatorProfile, error) {
	normalized := handle.Normalize(rawHandle)

	channel, err := s.channels.FindByHandle(ctx, normalized)
	if err != nil {
		if errors.Is(err, repository.ErrChannelNotFound) {
			return nil, ErrCreatorNotFound
		}

		return nil, fmt.Errorf("find channel: %w", err)
	}

	creator, err := s.creators.FindByID(ctx, channel.CreatorID)
	if err != nil {
		if errors.Is(err, repository.ErrCreatorNotFound) {
			return nil, ErrCreatorNotFound
		}

		return nil, fmt.Errorf("find creator: %w", err)
	}

	if creator.Status != model.CreatorStatusActive {
		return nil, ErrCreatorNotFound
	}

	return &CreatorProfile{
		Creator: creator,
		Channel: channel,
	}, nil
}

type UpdateCreatorInput struct {
	DisplayName *string
	Headline    *string
	Bio         *string
	WebsiteURL  *string
	CoverURL    *string
	ChannelName *string
	Description *string
	AvatarURL   *string
	BannerURL   *string
}

// UpdateCreator applies a partial update to the caller's creator and
// channel. Creator ID, user ID, handle, status, and timestamps are never
// client-writable. Avatar intent is singular until media-service exists:
// AvatarURL updates the public channel avatar and mirrors it onto the
// creator avatar.
func (s *CreatorService) UpdateCreator(
	ctx context.Context,
	userID uuid.UUID,
	input UpdateCreatorInput,
) (*CreatorProfile, error) {
	profile, err := s.GetMyCreator(ctx, userID)
	if err != nil {
		return nil, err
	}

	creator := profile.Creator
	channel := profile.Channel

	if input.DisplayName != nil {
		displayName := strings.TrimSpace(*input.DisplayName)

		if displayName == "" || utf8.RuneCountInString(displayName) > 100 {
			return nil, errors.New("display name must be 1-100 characters")
		}

		creator.DisplayName = displayName
	}

	if input.Headline != nil {
		headline := strings.TrimSpace(*input.Headline)

		if utf8.RuneCountInString(headline) > 150 {
			return nil, errors.New("headline must be at most 150 characters")
		}

		creator.Headline = nullable(headline)
	}

	if input.Bio != nil {
		bio := strings.TrimSpace(*input.Bio)

		if utf8.RuneCountInString(bio) > 1000 {
			return nil, errors.New("bio must be at most 1000 characters")
		}

		creator.Bio = nullable(bio)
	}

	if input.WebsiteURL != nil {
		website := strings.TrimSpace(*input.WebsiteURL)

		if website != "" {
			if err := validateWebsite(website); err != nil {
				return nil, err
			}
		}

		creator.WebsiteURL = nullable(website)
	}

	if input.CoverURL != nil {
		creator.CoverURL = nullable(strings.TrimSpace(*input.CoverURL))
	}

	if input.ChannelName != nil {
		name := strings.TrimSpace(*input.ChannelName)

		if name == "" || utf8.RuneCountInString(name) > 100 {
			return nil, errors.New("channel name must be 1-100 characters")
		}

		channel.Name = name
	}

	if input.Description != nil {
		description := strings.TrimSpace(*input.Description)

		if utf8.RuneCountInString(description) > 1000 {
			return nil, errors.New("description must be at most 1000 characters")
		}

		channel.Description = nullable(description)
	}

	if input.AvatarURL != nil {
		avatar := nullable(strings.TrimSpace(*input.AvatarURL))
		channel.AvatarURL = avatar
		creator.AvatarURL = avatar
	}

	if input.BannerURL != nil {
		channel.BannerURL = nullable(strings.TrimSpace(*input.BannerURL))
	}

	if err := s.creators.UpdateCreator(ctx, creator); err != nil {
		return nil, fmt.Errorf("update creator: %w", err)
	}

	if err := s.channels.UpdateChannel(ctx, channel); err != nil {
		return nil, fmt.Errorf("update channel: %w", err)
	}

	return &CreatorProfile{
		Creator: creator,
		Channel: channel,
	}, nil
}

func validateWebsite(website string) error {
	parsed, err := url.ParseRequestURI(website)
	if err != nil {
		return errors.New("invalid website URL")
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("invalid website URL")
	}

	if parsed.Host == "" {
		return errors.New("invalid website URL")
	}

	return nil
}

func nullable(value string) *string {
	if value == "" {
		return nil
	}

	return &value
}
