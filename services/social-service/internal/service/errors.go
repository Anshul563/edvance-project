package service

import (
	"errors"
)

var (
	ErrNotFound           = errors.New("resource not found")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
	ErrDuplicate          = errors.New("duplicate")
	ErrInvalidInput       = errors.New("invalid input")
	ErrInvalidContent     = errors.New("invalid content")
	ErrSelfFollow         = errors.New("cannot follow yourself")
	ErrInvalidParent      = errors.New("invalid parent comment")
	ErrPlaylistPrivate    = errors.New("playlist is private")
	ErrCreatorNotFound    = errors.New("creator not found")
	ErrContentNotFound    = errors.New("content not found")
	ErrNotificationFailed = errors.New("notification failed")
)
