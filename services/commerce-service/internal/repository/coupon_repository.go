package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/commerce-service/internal/model"
)

var (
	ErrCouponNotFound = errors.New("coupon not found")
)

type CouponRepository struct {
	db *pgxpool.Pool
}

func NewCouponRepository(db *pgxpool.Pool) *CouponRepository {
	return &CouponRepository{
		db: db,
	}
}

// CreateCoupon seeds coupon instruments. There is deliberately no public
// admin endpoint in v1: coupons are created by tests, migrations, and
// (later) an admin console.
func (r *CouponRepository) CreateCoupon(
	ctx context.Context,
	coupon *model.Coupon,
) error {
	query := `
		INSERT INTO coupons (
			code,
			discount_type,
			discount_value,
			currency,
			minimum_order_cents,
			maximum_discount_cents,
			usage_limit,
			starts_at,
			expires_at,
			status
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		coupon.Code,
		coupon.DiscountType,
		coupon.DiscountValue,
		coupon.Currency,
		coupon.MinimumOrderCents,
		coupon.MaximumDiscountCents,
		coupon.UsageLimit,
		coupon.StartsAt,
		coupon.ExpiresAt,
		coupon.Status,
	).Scan(&coupon.ID, &coupon.CreatedAt, &coupon.UpdatedAt)

	if err != nil {
		return fmt.Errorf("create coupon: %w", err)
	}

	return nil
}

func (r *CouponRepository) FindCouponByCode(
	ctx context.Context,
	code string,
) (*model.Coupon, error) {
	query := `
		SELECT
			id,
			code,
			discount_type,
			discount_value,
			currency,
			minimum_order_cents,
			maximum_discount_cents,
			usage_limit,
			used_count,
			starts_at,
			expires_at,
			status,
			created_at,
			updated_at
		FROM coupons
		WHERE code = $1
	`

	coupon := &model.Coupon{}

	err := r.db.QueryRow(ctx, query, code).Scan(
		&coupon.ID,
		&coupon.Code,
		&coupon.DiscountType,
		&coupon.DiscountValue,
		&coupon.Currency,
		&coupon.MinimumOrderCents,
		&coupon.MaximumDiscountCents,
		&coupon.UsageLimit,
		&coupon.UsedCount,
		&coupon.StartsAt,
		&coupon.ExpiresAt,
		&coupon.Status,
		&coupon.CreatedAt,
		&coupon.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCouponNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find coupon: %w", err)
	}

	return coupon, nil
}

func (r *CouponRepository) FindCouponByID(
	ctx context.Context,
	id uuid.UUID,
) (*model.Coupon, error) {
	coupon := &model.Coupon{}

	err := r.db.QueryRow(
		ctx,
		`SELECT
			id,
			code,
			discount_type,
			discount_value,
			currency,
			minimum_order_cents,
			maximum_discount_cents,
			usage_limit,
			used_count,
			starts_at,
			expires_at,
			status,
			created_at,
			updated_at
		FROM coupons
		WHERE id = $1`,
		id,
	).Scan(
		&coupon.ID,
		&coupon.Code,
		&coupon.DiscountType,
		&coupon.DiscountValue,
		&coupon.Currency,
		&coupon.MinimumOrderCents,
		&coupon.MaximumDiscountCents,
		&coupon.UsageLimit,
		&coupon.UsedCount,
		&coupon.StartsAt,
		&coupon.ExpiresAt,
		&coupon.Status,
		&coupon.CreatedAt,
		&coupon.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCouponNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find coupon: %w", err)
	}

	return coupon, nil
}
