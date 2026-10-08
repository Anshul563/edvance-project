package model

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// ContentKind identifies which content table a tag join table refers
// to. It is only ever constructed from typed constants — never parsed
// from client input — so it is safe to interpolate into SQL.
type ContentKind string

const (
	ContentKindVideo ContentKind = "video"
	ContentKindShort ContentKind = "short"
	ContentKindPost  ContentKind = "post"
)

func (k ContentKind) Valid() bool {
	switch k {
	case ContentKindVideo, ContentKindShort, ContentKindPost:
		return true
	default:
		return false
	}
}

// JoinTable returns the tag join table for this content kind.
func (k ContentKind) JoinTable() string {
	switch k {
	case ContentKindVideo:
		return "video_tags"
	case ContentKindShort:
		return "short_tags"
	case ContentKindPost:
		return "post_tags"
	default:
		return ""
	}
}

// Tag is a global, normalized taxonomy label. Name and slug are both
// derived from user input by NormalizeTag, so capitalization and
// punctuation differences can never produce duplicate tags: "React JS",
// "react js" and "  React   JS  " all normalize to name "react js" and
// slug "react-js".
type Tag struct {
	ID        uuid.UUID
	Name      string
	Slug      string
	CreatedAt time.Time
}

// NormalizeTag converts raw tag input into its canonical (name, slug)
// pair. ok is false when nothing usable remains (empty or punctuation
// only input).
func NormalizeTag(raw string) (name string, slug string, ok bool) {
	name = strings.ToLower(strings.Join(strings.Fields(raw), " "))
	name = strings.TrimSpace(name)

	if name == "" {
		return "", "", false
	}

	var builder strings.Builder

	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z',
			r >= '0' && r <= '9':
			builder.WriteRune(r)
		default:
			builder.WriteByte('-')
		}
	}

	slug = strings.Trim(builder.String(), "-")
	slug = collapseDashes(slug)

	if slug == "" {
		return "", "", false
	}

	return name, slug, true
}

func collapseDashes(value string) string {
	if !strings.Contains(value, "--") {
		return value
	}

	for strings.Contains(value, "--") {
		value = strings.ReplaceAll(value, "--", "-")
	}

	return value
}
