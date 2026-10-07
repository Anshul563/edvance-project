package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/commerce-service/internal/course"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/model"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/repository"
)

// CartStore is the persistence contract for carts.
// *repository.CartRepository satisfies it.
type CartStore interface {
	GetCartByUser(ctx context.Context, userID uuid.UUID) (*model.Cart, error)
	CreateCart(ctx context.Context, userID uuid.UUID) (*model.Cart, error)
	AddCartItem(ctx context.Context, cartID uuid.UUID, courseID uuid.UUID) error
	ListCartItems(ctx context.Context, cartID uuid.UUID) ([]*model.CartItem, error)
	RemoveCartItem(ctx context.Context, cartID uuid.UUID, courseID uuid.UUID) error
	ClearCart(ctx context.Context, cartID uuid.UUID) error
}

// CartService owns cart rules. Prices are always live from
// course-service; the cart stores relationships only, never prices.
type CartService struct {
	carts     CartStore
	courses   CourseStore
	purchases PurchaseLookup
}

func NewCartService(
	carts CartStore,
	courses CourseStore,
	purchases PurchaseLookup,
) (*CartService, error) {
	if carts == nil || courses == nil || purchases == nil {
		return nil, errors.New("cart dependencies are required")
	}

	return &CartService{
		carts:     carts,
		courses:   courses,
		purchases: purchases,
	}, nil
}

type CartView struct {
	CartID        uuid.UUID
	Lines         []CartLine
	SubtotalCents int64
	Currency      string
}

type CartLine struct {
	CourseID   uuid.UUID
	Title      string
	PriceCents int64
	Currency   string
}

// AddToCart validates saleability and ownership, then adds idempotently:
// repeats return the current cart state, never duplicates.
func (s *CartService) AddToCart(
	ctx context.Context,
	userID uuid.UUID,
	courseID uuid.UUID,
) (*CartView, error) {
	if userID == uuid.Nil || courseID == uuid.Nil {
		return nil, errors.New("user and course ids are required")
	}

	course, err := s.saleableCourse(ctx, courseID)
	if err != nil {
		return nil, err
	}

	if owned, err := s.ownedCourse(ctx, userID, courseID); err != nil {
		return nil, err
	} else if owned {
		return nil, ErrCourseAlreadyPurchased
	}

	cart, err := s.ensureCart(ctx, userID)
	if err != nil {
		return nil, err
	}

	if err := s.carts.AddCartItem(ctx, cart.ID, courseID); err != nil {
		if errors.Is(err, repository.ErrCartItemExists) {
			return s.viewCart(ctx, cart)
		}

		return nil, fmt.Errorf("add cart item: %w", err)
	}

	_ = course

	return s.viewCart(ctx, cart)
}

// GetCart returns the cart with live pricing. Courses that vanished
// upstream are skipped (the row stays for audit); a dead course-service
// fails the whole view rather than showing stale prices.
func (s *CartService) GetCart(
	ctx context.Context,
	userID uuid.UUID,
) (*CartView, error) {
	cart, err := s.ensureCart(ctx, userID)
	if err != nil {
		return nil, err
	}

	return s.viewCart(ctx, cart)
}

// RemoveCartItem deletes one line. Missing lines report not-found.
func (s *CartService) RemoveCartItem(
	ctx context.Context,
	userID uuid.UUID,
	courseID uuid.UUID,
) (*CartView, error) {
	cart, err := s.ensureCart(ctx, userID)
	if err != nil {
		return nil, err
	}

	if err := s.carts.RemoveCartItem(ctx, cart.ID, courseID); err != nil {
		if errors.Is(err, repository.ErrCartItemNotFound) {
			return nil, ErrCartItemNotFound
		}

		return nil, fmt.Errorf("remove cart item: %w", err)
	}

	return s.viewCart(ctx, cart)
}

// ClearCart empties the cart but keeps the cart row.
func (s *CartService) ClearCart(
	ctx context.Context,
	userID uuid.UUID,
) (*CartView, error) {
	cart, err := s.ensureCart(ctx, userID)
	if err != nil {
		return nil, err
	}

	if err := s.carts.ClearCart(ctx, cart.ID); err != nil {
		return nil, fmt.Errorf("clear cart: %w", err)
	}

	return s.viewCart(ctx, cart)
}

func (s *CartService) ensureCart(
	ctx context.Context,
	userID uuid.UUID,
) (*model.Cart, error) {
	cart, err := s.carts.GetCartByUser(ctx, userID)
	if err == nil {
		return cart, nil
	}

	if !errors.Is(err, repository.ErrCartNotFound) {
		return nil, fmt.Errorf("find cart: %w", err)
	}

	cart, err = s.carts.CreateCart(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrCartExists) {
			return s.carts.GetCartByUser(ctx, userID)
		}

		return nil, fmt.Errorf("create cart: %w", err)
	}

	return cart, nil
}

func (s *CartService) viewCart(
	ctx context.Context,
	cart *model.Cart,
) (*CartView, error) {
	items, err := s.carts.ListCartItems(ctx, cart.ID)
	if err != nil {
		return nil, fmt.Errorf("list cart items: %w", err)
	}

	view := &CartView{
		CartID:   cart.ID,
		Lines:    []CartLine{},
		Currency: cart.Currency,
	}

	for _, item := range items {
		c, err := s.courses.GetCourse(ctx, item.CourseID)
		if err != nil {
			if errors.Is(err, course.ErrCourseNotFound) {
				continue
			}

			return nil, ErrCourseUnavailable
		}

		view.Lines = append(view.Lines, CartLine{
			CourseID:   item.CourseID,
			Title:      c.Title,
			PriceCents: c.PriceCents,
			Currency:   c.Currency,
		})
		view.SubtotalCents += c.PriceCents
	}

	return view, nil
}

// saleableCourse fetches a course and enforces published + public.
func (s *CartService) saleableCourse(
	ctx context.Context,
	courseID uuid.UUID,
) (*course.Course, error) {
	course, err := s.courses.GetCourse(ctx, courseID)
	if err != nil {
		return nil, mapCourseError(err)
	}

	if !course.Saleable() {
		return nil, ErrCourseNotForSale
	}

	return course, nil
}

// ownedCourse reports active commercial ownership (refunded/revoked
// purchases do not block re-purchase).
func (s *CartService) ownedCourse(
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
