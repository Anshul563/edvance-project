package model

import (
	"time"

	"github.com/google/uuid"
)

// Category is a hierarchical taxonomy node. parent_id is a
// self-referencing foreign key: NULL means root category. Slugs are
// unique across the whole table so /categories/:slug stays unambiguous
// regardless of depth.
type Category struct {
	ID          uuid.UUID
	Name        string
	Slug        string
	Description *string
	ParentID    *uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
