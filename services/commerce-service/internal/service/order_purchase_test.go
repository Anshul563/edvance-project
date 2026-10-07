package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/commerce-service/internal/course"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/model"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/repository"
)

// fakeOrderStore is an in-memory OrderStore.
type fakeOrderStore struct {
	mu     sync.Mutex
	byID   map[uuid.UUID]*model.Order
	byNum  map[string]*model.Order
	items  map[uuid.UUID][]*model.OrderItem
	byUser map[uuid.UUID][]*model.Order
}

func newFakeOrderStore() *fakeOrderStore {
	return &fakeOrderStore{
		byID:   make(map[uuid.UUID]*model.Order),
		byNum:  make(map[string]*model.Order),
		items:  make(map[uuid.UUID][]*model.OrderItem),
		byUser: make(map[uuid.UUID][]*model.Order),
	}
}

func (f *fakeOrderStore) CreateOrderTx(
	_ context.Context,
	order *model.Order,
	items []repository.OrderItemInput,
	couponID *uuid.UUID,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if order.OrderNumber == "" {
		order.OrderNumber = "EDV-TEST"
	}

	if _, exists := f.byNum[order.OrderNumber]; exists {
		return repository.ErrOrderNumberTaken
	}

	if couponID != nil && *couponID == (uuid.UUID{}) {
		return repository.ErrCouponLimitReached
	}

	order.ID = uuid.New()

	stored := *order
	f.byID[order.ID] = &stored
	f.byNum[order.OrderNumber] = &stored
	f.byUser[order.UserID] = append(f.byUser[order.UserID], &stored)

	for _, input := range items {
		f.items[order.ID] = append(f.items[order.ID], &model.OrderItem{
			ID:              uuid.New(),
			OrderID:         order.ID,
			CourseID:        input.CourseID,
			CourseTitle:     input.CourseTitle,
			PriceCents:      input.PriceCents,
			DiscountCents:   input.DiscountCents,
			FinalPriceCents: input.FinalPriceCents,
			Currency:        input.Currency,
		})
	}

	return nil
}

func (f *fakeOrderStore) FindOrderByID(
	_ context.Context,
	id uuid.UUID,
) (*model.Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	order, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrOrderNotFound
	}

	cp := *order

	return &cp, nil
}

func (f *fakeOrderStore) ListOrderItems(
	_ context.Context,
	orderID uuid.UUID,
) ([]*model.OrderItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := []*model.OrderItem{}

	for _, item := range f.items[orderID] {
		cp := *item
		out = append(out, &cp)
	}

	return out, nil
}

func (f *fakeOrderStore) ListOrdersByUser(
	_ context.Context,
	userID uuid.UUID,
	status *model.OrderStatus,
	limit int,
	offset int,
) ([]*model.Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var all []*model.Order

	for _, order := range f.byUser[userID] {
		if status != nil && order.Status != *status {
			continue
		}

		cp := *order
		all = append(all, &cp)
	}

	if offset >= len(all) {
		return []*model.Order{}, nil
	}

	end := offset + limit
	if end > len(all) {
		end = len(all)
	}

	return all[offset:end], nil
}

func (f *fakeOrderStore) CountOrdersByUser(
	_ context.Context,
	userID uuid.UUID,
	status *model.OrderStatus,
) (int64, error) {
	items, _ := f.ListOrdersByUser(context.Background(), userID, status, 1<<30, 0)

	return int64(len(items)), nil
}

func (f *fakeOrderStore) UpdateOrderStatus(
	_ context.Context,
	id uuid.UUID,
	expected model.OrderStatus,
	next model.OrderStatus,
	paymentReference *string,
) (*model.Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	order, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrOrderNotFound
	}

	if order.Status != expected {
		return nil, repository.ErrInvalidOrderState
	}

	order.Status = next
	order.PaymentReference = paymentReference

	cp := *order

	return &cp, nil
}

// fakeFullPurchaseStore implements both PurchaseLookup and PurchaseStore.
type fakeFullPurchaseStore struct {
	fakePurchaseStore
	mu         sync.Mutex
	orders     map[uuid.UUID]*model.Order
	purchases  map[uuid.UUID][]*model.Purchase
	orderItems map[uuid.UUID][]uuid.UUID
}

func (f *fakeFullPurchaseStore) ListPurchasesByUser(
	_ context.Context,
	userID uuid.UUID,
	status *model.PurchaseStatus,
	limit int,
	offset int,
) ([]*model.Purchase, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	seen := map[uuid.UUID]bool{}
	var all []*model.Purchase

	collect := func(purchase *model.Purchase) {
		if purchase.UserID != userID {
			return
		}

		if status != nil && purchase.Status != *status {
			return
		}

		if seen[purchase.ID] {
			return
		}

		seen[purchase.ID] = true
		cp := *purchase
		all = append(all, &cp)
	}

	for _, list := range f.purchases {
		for _, purchase := range list {
			collect(purchase)
		}
	}

	// Embedded lookup map (seeded via own()).
	for _, byCourse := range f.fakePurchaseStore.purchases {
		for _, purchase := range byCourse {
			collect(purchase)
		}
	}

	if offset >= len(all) {
		return []*model.Purchase{}, nil
	}

	end := offset + limit
	if end > len(all) {
		end = len(all)
	}

	return all[offset:end], nil
}

func (f *fakeFullPurchaseStore) CountPurchasesByUser(
	_ context.Context,
	userID uuid.UUID,
	status *model.PurchaseStatus,
) (int64, error) {
	items, _ := f.ListPurchasesByUser(context.Background(), userID, status, 1<<30, 0)

	return int64(len(items)), nil
}

func (f *fakeFullPurchaseStore) CompleteOrderTx(
	_ context.Context,
	orderID uuid.UUID,
	paymentReference string,
) (*model.Order, []*model.Purchase, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	order, ok := f.orders[orderID]
	if !ok {
		return nil, nil, false, repository.ErrOrderNotFound
	}

	if order.Status == model.OrderPaid {
		return order, f.purchases[orderID], true, nil
	}

	if order.Status != model.OrderPendingPayment {
		return nil, nil, false, repository.ErrInvalidOrderState
	}

	order.Status = model.OrderPaid
	order.PaymentReference = &paymentReference

	created := []*model.Purchase{}

	for _, courseID := range f.orderCourses(orderID) {
		purchase := &model.Purchase{
			ID:       uuid.New(),
			UserID:   order.UserID,
			OrderID:  orderID,
			CourseID: courseID,
			Status:   model.PurchaseActive,
		}
		f.purchases[orderID] = append(f.purchases[orderID], purchase)
		created = append(created, purchase)
	}

	return order, created, false, nil
}

func (f *fakeFullPurchaseStore) orderCourses(orderID uuid.UUID) []uuid.UUID {
	return f.orderItems[orderID]
}

func (f *fakeFullPurchaseStore) seedOrder(
	order *model.Order,
	courseIDs []uuid.UUID,
) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.orders == nil {
		f.orders = make(map[uuid.UUID]*model.Order)
	}

	if f.purchases == nil {
		f.purchases = make(map[uuid.UUID][]*model.Purchase)
	}

	if f.orderItems == nil {
		f.orderItems = make(map[uuid.UUID][]uuid.UUID)
	}

	f.orders[order.ID] = order
	f.orderItems[order.ID] = courseIDs
}

// fakeProvisioner records provisioning calls with scripted failures.
type fakeProvisioner struct {
	mu       sync.Mutex
	calls    int
	failNext error
	provided map[uuid.UUID]map[uuid.UUID]bool
}

func (f *fakeProvisioner) ProvisionEnrollment(
	_ context.Context,
	userID uuid.UUID,
	courseID uuid.UUID,
	_ string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls++

	if f.failNext != nil {
		err := f.failNext
		f.failNext = nil

		return err
	}

	if f.provided == nil {
		f.provided = make(map[uuid.UUID]map[uuid.UUID]bool)
	}

	if f.provided[userID] == nil {
		f.provided[userID] = make(map[uuid.UUID]bool)
	}

	f.provided[userID][courseID] = true

	return nil
}

type orderFixture struct {
	orders    *OrderService
	purchases *PurchaseService
	orderRepo *fakeOrderStore
	purchase  *fakeFullPurchaseStore
	courses   *fakeCourseClient
	coupons   *fakeCouponStore
	provision *fakeProvisioner
}

func newOrderFixture() *orderFixture {
	orderRepo := newFakeOrderStore()
	purchaseRepo := &fakeFullPurchaseStore{
		fakePurchaseStore: fakePurchaseStore{
			purchases: make(map[uuid.UUID]map[uuid.UUID]*model.Purchase),
		},
		purchases:  make(map[uuid.UUID][]*model.Purchase),
		orders:     make(map[uuid.UUID]*model.Order),
		orderItems: make(map[uuid.UUID][]uuid.UUID),
	}
	courses := &fakeCourseClient{courses: make(map[uuid.UUID]*course.Course)}
	coupons := &fakeCouponStore{coupons: make(map[string]*model.Coupon)}
	provision := &fakeProvisioner{}

	couponSvc, err := NewCouponService(coupons, courses)
	if err != nil {
		panic(err)
	}

	orders, err := NewOrderService(
		orderRepo,
		courses,
		couponSvc,
		coupons,
		purchaseRepo,
	)
	if err != nil {
		panic(err)
	}

	purchaseSvc, err := NewPurchaseService(purchaseRepo, provision)
	if err != nil {
		panic(err)
	}

	return &orderFixture{
		orders:    orders,
		purchases: purchaseSvc,
		orderRepo: orderRepo,
		purchase:  purchaseRepo,
		courses:   courses,
		coupons:   coupons,
		provision: provision,
	}
}

func TestCreateOrderSingle(t *testing.T) {
	fx := newOrderFixture()
	ctx := context.Background()
	userID := uuid.New()
	courseID := uuid.New()
	fx.courses.courses[courseID] = saleableCourse(courseID, 99900)

	order, items, err := fx.orders.CreateOrder(ctx, userID, []uuid.UUID{courseID}, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if order.Status != model.OrderPendingPayment {
		t.Fatal("expected pending_payment")
	}

	if order.SubtotalCents != 99900 || order.TotalCents != 99900 {
		t.Fatalf("unexpected totals: %+v", order)
	}

	if len(order.OrderNumber) < 12 || order.OrderNumber[:4] != "EDV-" {
		t.Fatalf("bad order number %q", order.OrderNumber)
	}

	if len(items) != 1 || items[0].FinalPriceCents != 99900 {
		t.Fatal("snapshot mismatch")
	}

	if items[0].CourseTitle == "" {
		t.Fatal("title snapshot required")
	}
}

func TestCreateOrderMultiWithCoupon(t *testing.T) {
	fx := newOrderFixture()
	ctx := context.Background()
	userID := uuid.New()

	a := uuid.New()
	b := uuid.New()
	fx.courses.courses[a] = saleableCourse(a, 60000)
	fx.courses.courses[b] = saleableCourse(b, 40000)
	fx.coupons.coupons["WELCOME10"] = percentCoupon("WELCOME10", 1000)

	order, items, err := fx.orders.CreateOrder(
		ctx,
		userID,
		[]uuid.UUID{a, b, a},
		"welcome10",
	)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// 100000 subtotal, 10% = 10000 discount, total 90000.
	if order.SubtotalCents != 100000 || order.DiscountCents != 10000 || order.TotalCents != 90000 {
		t.Fatalf("unexpected totals: %+v", order)
	}

	var itemDiscount int64

	for _, item := range items {
		itemDiscount += item.DiscountCents
	}

	if itemDiscount != 10000 {
		t.Fatalf("item discounts must sum to 10000, got %d", itemDiscount)
	}

	if order.CouponCode == nil || *order.CouponCode != "WELCOME10" {
		t.Fatal("coupon code must be recorded normalized")
	}
}

func TestCreateOrderValidation(t *testing.T) {
	fx := newOrderFixture()
	ctx := context.Background()
	userID := uuid.New()

	ownedID := uuid.New()
	fx.courses.courses[ownedID] = saleableCourse(ownedID, 100)
	fx.purchase.own(userID, ownedID)

	draftID := uuid.New()
	draft := saleableCourse(draftID, 100)
	draft.Status = "draft"
	fx.courses.courses[draftID] = draft

	freeID := uuid.New()
	fx.courses.courses[freeID] = saleableCourse(freeID, 100)

	if _, _, err := fx.orders.CreateOrder(ctx, userID, nil, ""); !errors.Is(
		err,
		ErrEmptyOrder,
	) {
		t.Fatalf("expected empty order, got %v", err)
	}

	if _, _, err := fx.orders.CreateOrder(
		ctx,
		userID,
		[]uuid.UUID{ownedID},
		"",
	); !errors.Is(err, ErrCourseAlreadyPurchased) {
		t.Fatalf("expected already-purchased, got %v", err)
	}

	if _, _, err := fx.orders.CreateOrder(
		ctx,
		userID,
		[]uuid.UUID{draftID},
		"",
	); !errors.Is(err, ErrCourseNotForSale) {
		t.Fatalf("expected not-for-sale, got %v", err)
	}

	if _, _, err := fx.orders.CreateOrder(
		ctx,
		userID,
		[]uuid.UUID{uuid.New()},
		"",
	); !errors.Is(err, ErrCourseNotFound) {
		t.Fatalf("expected not-found, got %v", err)
	}

	if _, _, err := fx.orders.CreateOrder(
		ctx,
		userID,
		[]uuid.UUID{freeID},
		"NOPE",
	); !errors.Is(err, ErrCouponNotFound) {
		t.Fatalf("expected coupon not-found, got %v", err)
	}
}

func TestCompleteOrderFlow(t *testing.T) {
	fx := newOrderFixture()
	ctx := context.Background()
	userID := uuid.New()

	a := uuid.New()
	b := uuid.New()
	fx.courses.courses[a] = saleableCourse(a, 60000)
	fx.courses.courses[b] = saleableCourse(b, 40000)

	order, _, err := fx.orders.CreateOrder(ctx, userID, []uuid.UUID{a, b}, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Seed the fake purchase store's order view for completion.
	fx.purchase.seedOrder(
		&model.Order{ID: order.ID, UserID: userID, Status: model.OrderPendingPayment},
		[]uuid.UUID{a, b},
	)

	result, err := fx.purchases.CompleteOrder(ctx, order.ID, "pay_123")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}

	if result.Order.Status != model.OrderPaid {
		t.Fatal("expected paid")
	}

	if len(result.Purchases) != 2 {
		t.Fatalf("expected 2 purchases, got %d", len(result.Purchases))
	}

	if len(result.ProvisionErrors) != 0 {
		t.Fatal("provisioning should succeed")
	}

	if fx.provision.calls != 2 {
		t.Fatalf("expected 2 provisions, got %d", fx.provision.calls)
	}

	// Idempotent repeat: same state, provisions retried harmlessly.
	again, err := fx.purchases.CompleteOrder(ctx, order.ID, "pay_123")
	if err != nil {
		t.Fatalf("repeat: %v", err)
	}

	if !again.AlreadyDone || len(again.Purchases) != 2 {
		t.Fatal("expected idempotent replay")
	}

	// Non-pending states refuse.
	failed := &model.Order{ID: uuid.New(), UserID: userID, Status: model.OrderFailed}
	fx.purchase.seedOrder(failed, []uuid.UUID{a})

	if _, err := fx.purchases.CompleteOrder(ctx, failed.ID, "pay_x"); !errors.Is(
		err,
		ErrInvalidOrderState,
	) {
		t.Fatalf("expected invalid state, got %v", err)
	}

	if _, err := fx.purchases.CompleteOrder(
		ctx,
		uuid.New(),
		"pay_x",
	); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("expected not-found, got %v", err)
	}
}

func TestCompleteOrderProvisionFailure(t *testing.T) {
	fx := newOrderFixture()
	ctx := context.Background()
	userID := uuid.New()
	courseID := uuid.New()

	order := &model.Order{ID: uuid.New(), UserID: userID, Status: model.OrderPendingPayment}
	fx.purchase.seedOrder(order, []uuid.UUID{courseID})
	fx.provision.failNext = errors.New("learning down")

	result, err := fx.purchases.CompleteOrder(ctx, order.ID, "pay_1")
	if err != nil {
		t.Fatalf("complete must succeed despite provisioning: %v", err)
	}

	if len(result.ProvisionErrors) != 1 {
		t.Fatal("expected recorded provisioning failure")
	}

	if result.Order.Status != model.OrderPaid {
		t.Fatal("paid order must be preserved")
	}

	// Retry later succeeds.
	retry, err := fx.purchases.CompleteOrder(ctx, order.ID, "pay_1")
	if err != nil {
		t.Fatalf("retry: %v", err)
	}

	if len(retry.ProvisionErrors) != 0 {
		t.Fatal("retry should provision")
	}
}

func TestPurchaseHistory(t *testing.T) {
	fx := newOrderFixture()
	ctx := context.Background()
	userID := uuid.New()

	courseID := uuid.New()
	fx.purchase.own(userID, courseID)

	found, err := fx.purchases.GetPurchaseByCourse(ctx, userID, courseID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if found.Status != model.PurchaseActive {
		t.Fatal("expected active")
	}

	if _, err := fx.purchases.GetPurchaseByCourse(
		ctx,
		userID,
		uuid.New(),
	); !errors.Is(err, ErrPurchaseNotFound) {
		t.Fatalf("expected not-found, got %v", err)
	}

	page, err := fx.purchases.ListMyPurchases(ctx, userID, 1, 20, nil)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if page.Total != 1 {
		t.Fatalf("expected 1, got %d", page.Total)
	}
}
