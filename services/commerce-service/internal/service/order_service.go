package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/commerce-service/internal/course"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/model"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/repository"
)

// OrderStore is the persistence contract for orders.
// *repository.OrderRepository satisfies it.
type OrderStore interface {
	CreateOrderTx(
		ctx context.Context,
		order *model.Order,
		items []repository.OrderItemInput,
		couponID *uuid.UUID,
	) error
	FindOrderByID(ctx context.Context, id uuid.UUID) (*model.Order, error)
	ListOrderItems(ctx context.Context, orderID uuid.UUID) ([]*model.OrderItem, error)
	ListOrdersByUser(
		ctx context.Context,
		userID uuid.UUID,
		status *model.OrderStatus,
		limit int,
		offset int,
	) ([]*model.Order, error)
	CountOrdersByUser(
		ctx context.Context,
		userID uuid.UUID,
		status *model.OrderStatus,
	) (int64, error)
	UpdateOrderStatus(
		ctx context.Context,
		id uuid.UUID,
		expected model.OrderStatus,
		next model.OrderStatus,
		paymentReference *string,
	) (*model.Order, error)
}

// CouponValidator validates coupons without consuming them.
type CouponValidator interface {
	ValidateCoupon(
		ctx context.Context,
		code string,
		subtotalCents int64,
		currency string,
	) (*CouponQuote, error)
}

// CouponLookup resolves applied coupon codes to rows for usage
// accounting. *repository.CouponRepository satisfies it.
type CouponLookup interface {
	FindCouponByCode(ctx context.Context, code string) (*model.Coupon, error)
}

// OrderService owns pricing snapshots, totals, and order lifecycle. All
// commercial values are computed server-side from course-service reads:
// client-supplied prices, discounts, totals, and titles are never
// trusted.
type OrderService struct {
	orders    OrderStore
	courses   CourseStore
	coupons   CouponValidator
	couponRow CouponLookup
	purchases PurchaseLookup
}

func NewOrderService(
	orders OrderStore,
	courses CourseStore,
	coupons CouponValidator,
	couponRow CouponLookup,
	purchases PurchaseLookup,
) (*OrderService, error) {
	if orders == nil || courses == nil || purchases == nil {
		return nil, errors.New("order dependencies are required")
	}

	return &OrderService{
		orders:    orders,
		courses:   courses,
		coupons:   coupons,
		couponRow: couponRow,
		purchases: purchases,
	}, nil
}

type pricedCourse struct {
	course *course.Course
}

// CreateOrder validates courses, snapshots live prices, applies an
// optional coupon, and persists order + items + coupon usage in one
// transaction. Immutable snapshots mean later price changes can never
// rewrite history.
func (s *OrderService) CreateOrder(
	ctx context.Context,
	userID uuid.UUID,
	courseIDs []uuid.UUID,
	couponCode string,
) (*model.Order, []*model.OrderItem, error) {
	if userID == uuid.Nil {
		return nil, nil, errors.New("user id is required")
	}

	unique := dedupeUUIDs(courseIDs)

	if len(unique) == 0 {
		return nil, nil, ErrEmptyOrder
	}

	priced, err := s.priceCourses(ctx, userID, unique)
	if err != nil {
		return nil, nil, err
	}

	currency := priced[0].course.Currency
	subtotal := int64(0)

	for _, pc := range priced {
		if pc.course.Currency != currency {
			return nil, nil, ErrCurrencyMismatch
		}

		subtotal += pc.course.PriceCents
	}

	var quote *CouponQuote
	var couponID *uuid.UUID

	if code := strings.TrimSpace(couponCode); code != "" {
		if s.coupons == nil {
			return nil, nil, ErrInvalidCoupon
		}

		quote, err = s.coupons.ValidateCoupon(ctx, code, subtotal, currency)
		if err != nil {
			return nil, nil, err
		}

		if s.couponRow != nil {
			row, err := s.couponRow.FindCouponByCode(ctx, quote.Code)
			if err != nil {
				return nil, nil, fmt.Errorf("load coupon: %w", err)
			}

			couponID = &row.ID
		}
	}

	discount := int64(0)
	var couponCodePtr *string

	if quote != nil {
		discount = quote.DiscountCents
		couponCodePtr = &quote.Code
	}

	// Tax is a placeholder constant until a tax engine exists.
	const taxCents = int64(0)

	total := subtotal - discount + taxCents
	if total < 0 {
		total = 0
	}

	items := distributeDiscount(priced, discount, currency)

	order := &model.Order{
		UserID:        userID,
		Status:        model.OrderPendingPayment,
		Currency:      currency,
		SubtotalCents: subtotal,
		DiscountCents: discount,
		TaxCents:      taxCents,
		TotalCents:    total,
		CouponCode:    couponCodePtr,
	}

	for attempt := 0; attempt < 5; attempt++ {
		order.OrderNumber = generateOrderNumber()

		err := s.orders.CreateOrderTx(ctx, order, items, couponID)
		if err == nil {
			return s.loadOrderWithItems(ctx, order.ID)
		}

		if !errors.Is(err, repository.ErrOrderNumberTaken) &&
			!errors.Is(err, repository.ErrCouponLimitReached) {
			return nil, nil, fmt.Errorf("create order: %w", err)
		}

		if errors.Is(err, repository.ErrCouponLimitReached) {
			return nil, nil, ErrCouponLimitReached
		}
	}

	return nil, nil, errors.New("could not allocate order number")
}

// GetOrder returns an order with its items. Unknown or foreign orders
// read as not found: no ownership oracle.
func (s *OrderService) GetOrder(
	ctx context.Context,
	userID uuid.UUID,
	orderID uuid.UUID,
) (*model.Order, []*model.OrderItem, error) {
	order, err := s.ownedOrder(ctx, userID, orderID)
	if err != nil {
		return nil, nil, err
	}

	items, err := s.orders.ListOrderItems(ctx, orderID)
	if err != nil {
		return nil, nil, fmt.Errorf("list order items: %w", err)
	}

	return order, items, nil
}

type OrderPage struct {
	Items      []*model.Order
	Total      int64
	Page       int
	Limit      int
	TotalPages int64
}

// ListMyOrders returns the caller's orders, newest first.
func (s *OrderService) ListMyOrders(
	ctx context.Context,
	userID uuid.UUID,
	page int,
	limit int,
	status *model.OrderStatus,
) (*OrderPage, error) {
	if page < 1 {
		page = 1
	}

	if limit < 1 {
		limit = 20
	}

	if limit > 50 {
		limit = 50
	}

	items, err := s.orders.ListOrdersByUser(
		ctx,
		userID,
		status,
		limit,
		(page-1)*limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}

	total, err := s.orders.CountOrdersByUser(ctx, userID, status)
	if err != nil {
		return nil, fmt.Errorf("count orders: %w", err)
	}

	totalPages := int64(0)

	if total > 0 {
		totalPages = (total + int64(limit) - 1) / int64(limit)
	}

	return &OrderPage{
		Items:      items,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

// priceCourses fetches live courses and enforces saleability plus
// no-active-ownership per course.
func (s *OrderService) priceCourses(
	ctx context.Context,
	userID uuid.UUID,
	courseIDs []uuid.UUID,
) ([]pricedCourse, error) {
	priced := make([]pricedCourse, 0, len(courseIDs))

	for _, courseID := range courseIDs {
		course, err := s.courses.GetCourse(ctx, courseID)
		if err != nil {
			return nil, mapCourseError(err)
		}

		if !course.Saleable() {
			return nil, ErrCourseNotForSale
		}

		owned, err := s.ownedCourse(ctx, userID, courseID)
		if err != nil {
			return nil, err
		}

		if owned {
			return nil, ErrCourseAlreadyPurchased
		}

		priced = append(priced, pricedCourse{course: course})
	}

	return priced, nil
}

func (s *OrderService) ownedCourse(
	ctx context.Context,
	userID uuid.UUID,
	courseID uuid.UUID,
) (bool, error) {
	purchase, err := s.purchases.FindPurchaseByUserCourse(ctx, userID, courseID)
	if err != nil {
		if errors.Is(err, repository.ErrPurchaseNotFound) {
			return false, nil
		}

		return false, fmt.Errorf("check purchase: %w", err)
	}

	return purchase.Status == model.PurchaseActive, nil
}

func (s *OrderService) ownedOrder(
	ctx context.Context,
	userID uuid.UUID,
	orderID uuid.UUID,
) (*model.Order, error) {
	order, err := s.orders.FindOrderByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			return nil, ErrOrderNotFound
		}

		return nil, fmt.Errorf("find order: %w", err)
	}

	if order.UserID != userID {
		return nil, ErrOrderNotFound
	}

	return order, nil
}

func (s *OrderService) loadOrderWithItems(
	ctx context.Context,
	orderID uuid.UUID,
) (*model.Order, []*model.OrderItem, error) {
	order, err := s.orders.FindOrderByID(ctx, orderID)
	if err != nil {
		return nil, nil, fmt.Errorf("read order: %w", err)
	}

	items, err := s.orders.ListOrderItems(ctx, orderID)
	if err != nil {
		return nil, nil, fmt.Errorf("read order items: %w", err)
	}

	return order, items, nil
}

// distributeDiscount splits an order discount across items pro-rata in
// integer math (floor shares, remainder to the priciest line) so item
// discounts always sum exactly to the order discount.
func distributeDiscount(
	priced []pricedCourse,
	discount int64,
	currency string,
) []repository.OrderItemInput {
	items := make([]repository.OrderItemInput, 0, len(priced))

	var subtotal int64

	for _, pc := range priced {
		subtotal += pc.course.PriceCents
	}

	var assigned int64
	biggest := 0

	for i, pc := range priced {
		var share int64

		if subtotal > 0 && discount > 0 {
			share = pc.course.PriceCents * discount / subtotal
		}

		assigned += share

		if pc.course.PriceCents > priced[biggest].course.PriceCents {
			biggest = i
		}

		items = append(items, repository.OrderItemInput{
			CourseID:        pc.course.ID,
			CourseTitle:     pc.course.Title,
			PriceCents:      pc.course.PriceCents,
			DiscountCents:   share,
			FinalPriceCents: pc.course.PriceCents - share,
			Currency:        currency,
		})
	}

	remainder := discount - assigned

	if remainder != 0 && len(items) > 0 {
		items[biggest].DiscountCents += remainder
		items[biggest].FinalPriceCents -= remainder
	}

	return items
}

var orderNumberAlphabet = []byte("ABCDEFGHJKMNPQRSTUVWXYZ23456789")

// generateOrderNumber mints EDV-YYYYMMDD-XXXXXX identifiers: unique,
// human-friendly, non-sequential (no volume oracle), safe for display.
func generateOrderNumber() string {
	date := time.Now().Format("20060102")

	var suffix [6]byte

	if _, err := rand.Read(suffix[:]); err != nil {
		return "EDV-" + date + "-" + strings.ToUpper(uuid.NewString()[:6])
	}

	out := make([]byte, 6)

	for i, b := range suffix {
		out[i] = orderNumberAlphabet[int(b)%len(orderNumberAlphabet)]
	}

	return "EDV-" + date + "-" + string(out)
}

func dedupeUUIDs(ids []uuid.UUID) []uuid.UUID {
	seen := map[uuid.UUID]bool{}
	unique := make([]uuid.UUID, 0, len(ids))

	for _, id := range ids {
		if id == uuid.Nil || seen[id] {
			continue
		}

		seen[id] = true
		unique = append(unique, id)
	}

	return unique
}
