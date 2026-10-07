package course

import (
	"github.com/google/uuid"
)

// Course mirrors the course-service course representation (subset
// actually needed: identity, saleability, and live pricing).
type Course struct {
	ID         uuid.UUID
	CreatorID  uuid.UUID
	Title      string
	Slug       string
	Status     string
	Visibility string
	PriceCents int64
	Currency   string
}

// Saleable reports whether the course can be bought right now.
func (c *Course) Saleable() bool {
	return c.Status == "published" && c.Visibility == "public"
}
