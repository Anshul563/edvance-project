package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/commerce-service/internal/model"
)

var (
	ErrOrderNotFound       = errors.New("order not found")
	ErrOrderNumberTaken    = errors.New("order number already taken")
	ErrCouponLimitReached  = errors.New("coupon usage limit reached")
	ErrInvalidOrderState   = errors.New("invalid order state")
	ErrIdempotencyKeyTaken = errors.New("order idempotency key already taken")
)

const orderColumns = `
	id,
	user_id,
	order_number,
	idempotency_key,
	request_fingerprint,
	status,
	currency,
	subtotal_cents,
	discount_cents,
	tax_cents,
	total_cents,
	coupon_code,
	payment_reference,
	created_at,
	updated_at,
	completed_at,
	cancelled_at
`

type OrderRepository struct {
	db *pgxpool.Pool
}

func NewOrderRepository(db *pgxpool.Pool) *OrderRepository {
	return &OrderRepository{
		db: db,
	}
}

type OrderItemInput struct {
	CourseID        uuid.UUID
	CourseTitle     string
	PriceCents      int64
	DiscountCents   int64
	FinalPriceCents int64
	Currency        string
}

// CreateOrderTx inserts the order, its immutable item snapshots, and —
// when a coupon applies — consumes one usage slot, all atomically. The
// coupon increment is conditional on remaining capacity, so concurrent
// checkouts race safely into ErrCouponLimitReached instead of
// overshooting the limit. Coupon usage is recorded only here: a failed
// order creation never consumes a slot.
func (r *OrderRepository) CreateOrderTx(
	ctx context.Context,
	order *model.Order,
	items []OrderItemInput,
	couponID *uuid.UUID,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("create order: begin: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	err = tx.QueryRow(
		ctx,
		`INSERT INTO orders (
			user_id,
			order_number,
			idempotency_key,
			request_fingerprint,
			status,
			currency,
			subtotal_cents,
			discount_cents,
			tax_cents,
			total_cents,
			coupon_code
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at`,
		order.UserID,
		order.OrderNumber,
		order.IdempotencyKey,
		order.RequestFingerprint,
		order.Status,
		order.Currency,
		order.SubtotalCents,
		order.DiscountCents,
		order.TaxCents,
		order.TotalCents,
		order.CouponCode,
	).Scan(&order.ID, &order.CreatedAt, &order.UpdatedAt)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if strings.Contains(pgErr.ConstraintName, "order_number") {
				return ErrOrderNumberTaken
			}
			if strings.Contains(pgErr.ConstraintName, "idempotency") {
				return ErrIdempotencyKeyTaken
			}

			return fmt.Errorf("create order: %w", err)
		}

		return fmt.Errorf("create order: %w", err)
	}

	for _, item := range items {
		if _, err := tx.Exec(
			ctx,
			`INSERT INTO order_items (
				order_id,
				course_id,
				course_title_snapshot,
				price_cents,
				discount_cents,
				final_price_cents,
				currency
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			order.ID,
			item.CourseID,
			item.CourseTitle,
			item.PriceCents,
			item.DiscountCents,
			item.FinalPriceCents,
			item.Currency,
		); err != nil {
			return fmt.Errorf("create order: items: %w", err)
		}
	}

	if couponID != nil {
		tag, err := tx.Exec(
			ctx,
			`UPDATE coupons
			 SET used_count = used_count + 1, updated_at = NOW()
			 WHERE id = $1
				AND (usage_limit IS NULL OR used_count < usage_limit)`,
			*couponID,
		)
		if err != nil {
			return fmt.Errorf("create order: coupon: %w", err)
		}

		if tag.RowsAffected() == 0 {
			return ErrCouponLimitReached
		}

		if _, err := tx.Exec(
			ctx,
			`INSERT INTO coupon_usages (coupon_id, user_id, order_id)
			 VALUES ($1, $2, $3)`,
			*couponID,
			order.UserID,
			order.ID,
		); err != nil {
			return fmt.Errorf("create order: coupon usage: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("create order: commit: %w", err)
	}

	return nil
}

func (r *OrderRepository) FindOrderByUserIdempotencyKey(
	ctx context.Context,
	userID uuid.UUID,
	key string,
) (*model.Order, error) {
	order := &model.Order{}
	err := r.db.QueryRow(ctx,
		`SELECT `+orderColumns+` FROM orders WHERE user_id = $1 AND idempotency_key = $2`,
		userID,
		key,
	).Scan(scanOrderArgs(order)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find order by idempotency key: %w", err)
	}
	return order, nil
}

func (r *OrderRepository) FindOrderByID(
	ctx context.Context,
	id uuid.UUID,
) (*model.Order, error) {
	query := `
		SELECT ` + orderColumns + `
		FROM orders
		WHERE id = $1
	`

	order := &model.Order{}

	err := r.db.QueryRow(ctx, query, id).Scan(scanOrderArgs(order)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOrderNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find order: %w", err)
	}

	return order, nil
}

func (r *OrderRepository) ListOrderItems(
	ctx context.Context,
	orderID uuid.UUID,
) ([]*model.OrderItem, error) {
	rows, err := r.db.Query(
		ctx,
		`SELECT
			id,
			order_id,
			course_id,
			course_title_snapshot,
			price_cents,
			discount_cents,
			final_price_cents,
			currency,
			created_at
		FROM order_items
		WHERE order_id = $1
		ORDER BY created_at ASC`,
		orderID,
	)
	if err != nil {
		return nil, fmt.Errorf("list order items: %w", err)
	}
	defer rows.Close()

	items := []*model.OrderItem{}

	for rows.Next() {
		item := &model.OrderItem{}

		if err := rows.Scan(
			&item.ID,
			&item.OrderID,
			&item.CourseID,
			&item.CourseTitle,
			&item.PriceCents,
			&item.DiscountCents,
			&item.FinalPriceCents,
			&item.Currency,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan order item: %w", err)
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list order items: %w", err)
	}

	return items, nil
}

func (r *OrderRepository) ListOrdersByUser(
	ctx context.Context,
	userID uuid.UUID,
	status *model.OrderStatus,
	limit int,
	offset int,
) ([]*model.Order, error) {
	query := `
		SELECT ` + orderColumns + `
		FROM orders
		WHERE user_id = $1
	`

	args := []any{userID}
	pos := 2

	if status != nil {
		query += fmt.Sprintf(" AND status = $%d", pos)
		args = append(args, *status)
		pos++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", pos, pos+1)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()

	orders := []*model.Order{}

	for rows.Next() {
		order := &model.Order{}

		if err := rows.Scan(scanOrderArgs(order)...); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}

		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}

	return orders, nil
}

func (r *OrderRepository) CountOrdersByUser(
	ctx context.Context,
	userID uuid.UUID,
	status *model.OrderStatus,
) (int64, error) {
	query := `
		SELECT COUNT(*)
		FROM orders
		WHERE user_id = $1
	`

	args := []any{userID}

	if status != nil {
		query += ` AND status = $2`
		args = append(args, *status)
	}

	var total int64

	if err := r.db.QueryRow(ctx, query, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count orders: %w", err)
	}

	return total, nil
}

// CanTransition is the single source of truth for the order lifecycle:
//
//	pending_payment -> paid, failed, cancelled
//	paid            -> refunded, partially_refunded
//
// Terminal states (failed, cancelled, refunded, partially_refunded)
// never move. UpdateOrderStatus enforces this table alongside its
// compare-and-swap write.
func CanTransition(from model.OrderStatus, to model.OrderStatus) bool {
	switch from {
	case model.OrderPendingPayment:
		return to == model.OrderPaid ||
			to == model.OrderFailed ||
			to == model.OrderCancelled

	case model.OrderPaid:
		return to == model.OrderRefunded ||
			to == model.OrderPartiallyRefunded

	case model.OrderPartiallyRefunded:
		return to == model.OrderRefunded
	}

	return false
}

// UpdateOrderStatus moves an order from one expected state to another.
// Both guards apply: the transition must be legal per CanTransition and
// the current state must match expected (compare-and-swap). Violations
// report ErrInvalidOrderState instead of transitioning.
func (r *OrderRepository) UpdateOrderStatus(
	ctx context.Context,
	id uuid.UUID,
	expected model.OrderStatus,
	next model.OrderStatus,
	paymentReference *string,
) (*model.Order, error) {
	if !CanTransition(expected, next) {
		return nil, ErrInvalidOrderState
	}

	query := `
		UPDATE orders
		SET
			status = $3::varchar,
			payment_reference = COALESCE($4::text, payment_reference),
			completed_at = CASE WHEN $3::varchar = 'paid' THEN NOW() ELSE completed_at END,
			cancelled_at = CASE WHEN $3::varchar = 'cancelled' THEN NOW() ELSE cancelled_at END,
			updated_at = NOW()
		WHERE id = $1 AND status = $2::varchar
		RETURNING ` + orderColumns

	order := &model.Order{}

	err := r.db.QueryRow(
		ctx,
		query,
		id,
		expected,
		next,
		paymentReference,
	).Scan(scanOrderArgs(order)...)

	if errors.Is(err, pgx.ErrNoRows) {
		if _, findErr := r.FindOrderByID(ctx, id); findErr != nil {
			return nil, findErr
		}

		return nil, ErrInvalidOrderState
	}

	if err != nil {
		return nil, fmt.Errorf("update order status: %w", err)
	}

	return order, nil
}

func scanOrderArgs(order *model.Order) []any {
	return []any{
		&order.ID,
		&order.UserID,
		&order.OrderNumber,
		&order.IdempotencyKey,
		&order.RequestFingerprint,
		&order.Status,
		&order.Currency,
		&order.SubtotalCents,
		&order.DiscountCents,
		&order.TaxCents,
		&order.TotalCents,
		&order.CouponCode,
		&order.PaymentReference,
		&order.CreatedAt,
		&order.UpdatedAt,
		&order.CompletedAt,
		&order.CancelledAt,
	}
}
