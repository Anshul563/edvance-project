package model

import (
	"time"

	"github.com/google/uuid"
)

// Cart is one row per user. Prices are intentionally NOT stored here:
// the cart always prices live from course-service, while orders freeze
// immutable snapshots at purchase time.
type Cart struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Currency  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CartItem links a cart to a course. CourseID is a cross-service
// identifier (plain UUID, never a foreign key).
type CartItem struct {
	ID        uuid.UUID
	CartID    uuid.UUID
	CourseID  uuid.UUID
	CreatedAt time.Time
}

// CartLine pairs an item with its live course data for display.
type CartLine struct {
	Item     *CartItem
	CourseID uuid.UUID
	Title    string
	Price    int64
	Currency string
}
