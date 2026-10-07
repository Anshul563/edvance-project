package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/commerce-service/internal/model"
)

var (
	ErrPurchaseNotFound = errors.New("purchase not found")
	ErrPurchaseExists   = errors.New("purchase already exists")
)

type PurchaseRepository struct {
	db *pgxpool.Pool
}

func NewPurchaseRepository(db *pgxpool.Pool) *PurchaseRepository {
	return &PurchaseRepository{
		db: db,
	}
}

func (r *PurchaseRepository) CreatePurchase(
	ctx context.Context,
	purchase *model.Purchase,
) error {
	query := `
		INSERT INTO purchases (
			user_id,
			order_id,
			course_id,
			status
		)
		VALUES ($1, $2, $3, $4)
		RETURNING id, purchased_at, created_at, updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		purchase.UserID,
		purchase.OrderID,
		purchase.CourseID,
		purchase.Status,
	).Scan(
		&purchase.ID,
		&purchase.PurchasedAt,
		&purchase.CreatedAt,
		&purchase.UpdatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrPurchaseExists
		}

		return fmt.Errorf("create purchase: %w", err)
	}

	return nil
}

func (r *PurchaseRepository) FindPurchaseByUserCourse(
	ctx context.Context,
	userID uuid.UUID,
	courseID uuid.UUID,
) (*model.Purchase, error) {
	query := `
		SELECT
			id,
			user_id,
			order_id,
			course_id,
			status,
			purchased_at,
			refunded_at,
			created_at,
			updated_at
		FROM purchases
		WHERE user_id = $1 AND course_id = $2
		ORDER BY created_at DESC
		LIMIT 1
	`

	purchase := &model.Purchase{}

	err := r.db.QueryRow(ctx, query, userID, courseID).Scan(
		&purchase.ID,
		&purchase.UserID,
		&purchase.OrderID,
		&purchase.CourseID,
		&purchase.Status,
		&purchase.PurchasedAt,
		&purchase.RefundedAt,
		&purchase.CreatedAt,
		&purchase.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPurchaseNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find purchase: %w", err)
	}

	return purchase, nil
}

func (r *PurchaseRepository) ListPurchasesByUser(
	ctx context.Context,
	userID uuid.UUID,
	status *model.PurchaseStatus,
	limit int,
	offset int,
) ([]*model.Purchase, error) {
	query := `
		SELECT
			id,
			user_id,
			order_id,
			course_id,
			status,
			purchased_at,
			refunded_at,
			created_at,
			updated_at
		FROM purchases
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
		return nil, fmt.Errorf("list purchases: %w", err)
	}
	defer rows.Close()

	purchases := []*model.Purchase{}

	for rows.Next() {
		purchase := &model.Purchase{}

		if err := rows.Scan(
			&purchase.ID,
			&purchase.UserID,
			&purchase.OrderID,
			&purchase.CourseID,
			&purchase.Status,
			&purchase.PurchasedAt,
			&purchase.RefundedAt,
			&purchase.CreatedAt,
			&purchase.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan purchase: %w", err)
		}

		purchases = append(purchases, purchase)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list purchases: %w", err)
	}

	return purchases, nil
}

func (r *PurchaseRepository) CountPurchasesByUser(
	ctx context.Context,
	userID uuid.UUID,
	status *model.PurchaseStatus,
) (int64, error) {
	query := `
		SELECT COUNT(*)
		FROM purchases
		WHERE user_id = $1
	`

	args := []any{userID}

	if status != nil {
		query += ` AND status = $2`
		args = append(args, *status)
	}

	var total int64

	if err := r.db.QueryRow(ctx, query, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count purchases: %w", err)
	}

	return total, nil
}

func (r *PurchaseRepository) ListPurchasesByOrder(
	ctx context.Context,
	orderID uuid.UUID,
) ([]*model.Purchase, error) {
	rows, err := r.db.Query(
		ctx,
		`SELECT
			id,
			user_id,
			order_id,
			course_id,
			status,
			purchased_at,
			refunded_at,
			created_at,
			updated_at
		FROM purchases
		WHERE order_id = $1
		ORDER BY created_at ASC`,
		orderID,
	)
	if err != nil {
		return nil, fmt.Errorf("list order purchases: %w", err)
	}
	defer rows.Close()

	purchases := []*model.Purchase{}

	for rows.Next() {
		purchase := &model.Purchase{}

		if err := rows.Scan(
			&purchase.ID,
			&purchase.UserID,
			&purchase.OrderID,
			&purchase.CourseID,
			&purchase.Status,
			&purchase.PurchasedAt,
			&purchase.RefundedAt,
			&purchase.CreatedAt,
			&purchase.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan purchase: %w", err)
		}

		purchases = append(purchases, purchase)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list order purchases: %w", err)
	}

	return purchases, nil
}

// CompleteOrderTx finalizes payment: pending -> paid with timestamp and
// payment reference, plus one purchase per order item — all atomically.
// A paid order replays idempotently (existing purchases returned, no
// duplicates); any other non-pending state refuses. The caller (payment
// completion) provisions enrollments afterwards from the returned
// purchases; a provisioning outage never rolls back this transaction.
func (r *PurchaseRepository) CompleteOrderTx(
	ctx context.Context,
	orderID uuid.UUID,
	paymentReference string,
) (*model.Order, []*model.Purchase, bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, nil, false, fmt.Errorf("complete order: begin: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	order := &model.Order{}

	err = tx.QueryRow(
		ctx,
		`SELECT `+orderColumns+`
		 FROM orders
		 WHERE id = $1
		 FOR UPDATE`,
		orderID,
	).Scan(scanOrderArgs(order)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, false, ErrOrderNotFound
	}

	if err != nil {
		return nil, nil, false, fmt.Errorf("complete order: lock: %w", err)
	}

	if order.Status == model.OrderPaid {
		purchases, err := r.listOrderPurchasesTx(ctx, tx, orderID)
		if err != nil {
			return nil, nil, false, err
		}

		if err := tx.Commit(ctx); err != nil {
			return nil, nil, false, fmt.Errorf("complete order: commit: %w", err)
		}

		return order, purchases, true, nil
	}

	if order.Status != model.OrderPendingPayment {
		return nil, nil, false, ErrInvalidOrderState
	}

	now := timeNow()

	if _, err := tx.Exec(
		ctx,
		`UPDATE orders
		 SET status = $2, payment_reference = $3, completed_at = $4, updated_at = $4
		 WHERE id = $1`,
		orderID,
		model.OrderPaid,
		paymentReference,
		now,
	); err != nil {
		return nil, nil, false, fmt.Errorf("complete order: update: %w", err)
	}

	rows, err := tx.Query(
		ctx,
		`SELECT course_id FROM order_items WHERE order_id = $1 ORDER BY created_at ASC`,
		orderID,
	)
	if err != nil {
		return nil, nil, false, fmt.Errorf("complete order: items: %w", err)
	}

	var courseIDs []uuid.UUID

	for rows.Next() {
		var courseID uuid.UUID

		if err := rows.Scan(&courseID); err != nil {
			rows.Close()
			return nil, nil, false, fmt.Errorf("complete order: scan: %w", err)
		}

		courseIDs = append(courseIDs, courseID)
	}

	rows.Close()

	if err := rows.Err(); err != nil {
		return nil, nil, false, fmt.Errorf("complete order: rows: %w", err)
	}

	purchases := make([]*model.Purchase, 0, len(courseIDs))

	for _, courseID := range courseIDs {
		purchase := &model.Purchase{}

		err := tx.QueryRow(
			ctx,
			`INSERT INTO purchases (user_id, order_id, course_id, status)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (order_id, course_id) DO NOTHING
			 RETURNING id, purchased_at, created_at, updated_at`,
			order.UserID,
			orderID,
			courseID,
			model.PurchaseActive,
		).Scan(
			&purchase.ID,
			&purchase.PurchasedAt,
			&purchase.CreatedAt,
			&purchase.UpdatedAt,
		)

		if errors.Is(err, pgx.ErrNoRows) {
			// Lost an insert race: adopt the winner's row.
			err = tx.QueryRow(
				ctx,
				`SELECT
					id,
					user_id,
					order_id,
					course_id,
					status,
					purchased_at,
					refunded_at,
					created_at,
					updated_at
				FROM purchases
				WHERE order_id = $1 AND course_id = $2`,
				orderID,
				courseID,
			).Scan(
				&purchase.ID,
				&purchase.UserID,
				&purchase.OrderID,
				&purchase.CourseID,
				&purchase.Status,
				&purchase.PurchasedAt,
				&purchase.RefundedAt,
				&purchase.CreatedAt,
				&purchase.UpdatedAt,
			)
		}

		if err != nil {
			return nil, nil, false, fmt.Errorf("complete order: purchase: %w", err)
		}

		purchase.UserID = order.UserID
		purchase.OrderID = orderID
		purchase.CourseID = courseID

		if purchase.Status == "" {
			purchase.Status = model.PurchaseActive
		}

		purchases = append(purchases, purchase)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, false, fmt.Errorf("complete order: commit: %w", err)
	}

	order.Status = model.OrderPaid
	order.PaymentReference = &paymentReference
	order.CompletedAt = &now
	order.UpdatedAt = now

	return order, purchases, false, nil
}

func (r *PurchaseRepository) listOrderPurchasesTx(
	ctx context.Context,
	tx pgx.Tx,
	orderID uuid.UUID,
) ([]*model.Purchase, error) {
	rows, err := tx.Query(
		ctx,
		`SELECT
			id,
			user_id,
			order_id,
			course_id,
			status,
			purchased_at,
			refunded_at,
			created_at,
			updated_at
		FROM purchases
		WHERE order_id = $1
		ORDER BY created_at ASC`,
		orderID,
	)
	if err != nil {
		return nil, fmt.Errorf("complete order: purchases: %w", err)
	}
	defer rows.Close()

	purchases := []*model.Purchase{}

	for rows.Next() {
		purchase := &model.Purchase{}

		if err := rows.Scan(
			&purchase.ID,
			&purchase.UserID,
			&purchase.OrderID,
			&purchase.CourseID,
			&purchase.Status,
			&purchase.PurchasedAt,
			&purchase.RefundedAt,
			&purchase.CreatedAt,
			&purchase.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("complete order: scan: %w", err)
		}

		purchases = append(purchases, purchase)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("complete order: rows: %w", err)
	}

	return purchases, nil
}

func timeNow() time.Time {
	return time.Now()
}
