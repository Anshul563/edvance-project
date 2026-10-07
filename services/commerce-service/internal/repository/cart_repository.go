package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/commerce-service/internal/model"
)

var (
	ErrCartNotFound     = errors.New("cart not found")
	ErrCartExists       = errors.New("cart already exists")
	ErrCartItemExists   = errors.New("cart item already exists")
	ErrCartItemNotFound = errors.New("cart item not found")
)

type CartRepository struct {
	db *pgxpool.Pool
}

func NewCartRepository(db *pgxpool.Pool) *CartRepository {
	return &CartRepository{
		db: db,
	}
}

func (r *CartRepository) GetCartByUser(
	ctx context.Context,
	userID uuid.UUID,
) (*model.Cart, error) {
	query := `
		SELECT id, user_id, currency, created_at, updated_at
		FROM carts
		WHERE user_id = $1
	`

	cart := &model.Cart{}

	err := r.db.QueryRow(ctx, query, userID).Scan(
		&cart.ID,
		&cart.UserID,
		&cart.Currency,
		&cart.CreatedAt,
		&cart.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCartNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find cart: %w", err)
	}

	return cart, nil
}

func (r *CartRepository) CreateCart(
	ctx context.Context,
	userID uuid.UUID,
) (*model.Cart, error) {
	query := `
		INSERT INTO carts (user_id)
		VALUES ($1)
		RETURNING id, user_id, currency, created_at, updated_at
	`

	cart := &model.Cart{}

	err := r.db.QueryRow(ctx, query, userID).Scan(
		&cart.ID,
		&cart.UserID,
		&cart.Currency,
		&cart.CreatedAt,
		&cart.UpdatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrCartExists
		}

		return nil, fmt.Errorf("create cart: %w", err)
	}

	return cart, nil
}

func (r *CartRepository) AddCartItem(
	ctx context.Context,
	cartID uuid.UUID,
	courseID uuid.UUID,
) error {
	_, err := r.db.Exec(
		ctx,
		`INSERT INTO cart_items (cart_id, course_id) VALUES ($1, $2)`,
		cartID,
		courseID,
	)
	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrCartItemExists
		}

		return fmt.Errorf("add cart item: %w", err)
	}

	return nil
}

func (r *CartRepository) ListCartItems(
	ctx context.Context,
	cartID uuid.UUID,
) ([]*model.CartItem, error) {
	rows, err := r.db.Query(
		ctx,
		`SELECT id, cart_id, course_id, created_at
		 FROM cart_items
		 WHERE cart_id = $1
		 ORDER BY created_at ASC`,
		cartID,
	)
	if err != nil {
		return nil, fmt.Errorf("list cart items: %w", err)
	}
	defer rows.Close()

	items := []*model.CartItem{}

	for rows.Next() {
		item := &model.CartItem{}

		if err := rows.Scan(
			&item.ID,
			&item.CartID,
			&item.CourseID,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan cart item: %w", err)
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list cart items: %w", err)
	}

	return items, nil
}

func (r *CartRepository) RemoveCartItem(
	ctx context.Context,
	cartID uuid.UUID,
	courseID uuid.UUID,
) error {
	tag, err := r.db.Exec(
		ctx,
		`DELETE FROM cart_items WHERE cart_id = $1 AND course_id = $2`,
		cartID,
		courseID,
	)
	if err != nil {
		return fmt.Errorf("remove cart item: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrCartItemNotFound
	}

	return nil
}

// ClearCart removes all items but keeps the cart row: carts are cheap,
// stable anchors for the user.
func (r *CartRepository) ClearCart(
	ctx context.Context,
	cartID uuid.UUID,
) error {
	if _, err := r.db.Exec(
		ctx,
		`DELETE FROM cart_items WHERE cart_id = $1`,
		cartID,
	); err != nil {
		return fmt.Errorf("clear cart: %w", err)
	}

	return nil
}
