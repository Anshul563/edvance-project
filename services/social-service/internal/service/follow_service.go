package service

import (
	"context"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/social-service/internal/model"
	"github.com/Anshul563/edvance-project/services/social-service/internal/repository"
)

type FollowService struct {
	follows *repository.FollowRepository
}

func NewFollowService(follows *repository.FollowRepository) *FollowService {
	return &FollowService{follows: follows}
}

func (s *FollowService) Follow(ctx context.Context, followerID, followingID uuid.UUID) error {
	if followerID == followingID {
		return ErrSelfFollow
	}

	_, created, err := s.follows.Create(ctx, &model.Follow{
		FollowerID:  followerID,
		FollowingID: followingID,
	})
	if err != nil {
		return err
	}

	if !created {
		return ErrDuplicate
	}
	return nil
}

func (s *FollowService) Unfollow(ctx context.Context, followerID, followingID uuid.UUID) error {
	_, err := s.follows.Delete(ctx, followerID, followingID)
	if err != nil {
		return err
	}
	return nil
}

func (s *FollowService) IsFollowing(ctx context.Context, followerID, followingID uuid.UUID) (bool, error) {
	return s.follows.IsFollowing(ctx, followerID, followingID)
}

func (s *FollowService) Count(ctx context.Context, userID uuid.UUID) (followers, following int64, err error) {
	followers, err = s.follows.CountFollowers(ctx, userID)
	if err != nil {
		return 0, 0, err
	}
	following, err = s.follows.CountFollowing(ctx, userID)
	if err != nil {
		return 0, 0, err
	}
	return followers, following, nil
}

func (s *FollowService) ListFollowers(ctx context.Context, userID uuid.UUID, offset, limit int) ([]model.Follow, error) {
	return s.follows.ListFollowers(ctx, userID, offset, limit)
}

func (s *FollowService) ListFollowing(ctx context.Context, userID uuid.UUID, offset, limit int) ([]model.Follow, error) {
	return s.follows.ListFollowing(ctx, userID, offset, limit)
}
