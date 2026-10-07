package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/payment-service/internal/commerce"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/model"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/razorpay"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/repository"
)

// fakePaymentStore is an in-memory PaymentStore mirroring repository
// conditional semantics.
type fakePaymentStore struct {
	mu   sync.Mutex
	byID map[uuid.UUID]*model.Payment
	byCO map[uuid.UUID]*model.Payment
	byPO map[string]*model.Payment
	byPP map[string]*model.Payment
}

func newFakePaymentStore() *fakePaymentStore {
	return &fakePaymentStore{
		byID: make(map[uuid.UUID]*model.Payment),
		byCO: make(map[uuid.UUID]*model.Payment),
		byPO: make(map[string]*model.Payment),
		byPP: make(map[string]*model.Payment),
	}
}

func (f *fakePaymentStore) CreatePayment(
	_ context.Context,
	payment *model.Payment,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	payment.ID = uuid.New()
	payment.CreatedAt = time.Now()
	payment.UpdatedAt = time.Now()

	stored := *payment
	f.byID[payment.ID] = &stored
	f.byCO[payment.CommerceOrderID] = &stored

	return nil
}

func (f *fakePaymentStore) FindPaymentByID(
	_ context.Context,
	id uuid.UUID,
) (*model.Payment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	payment, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrPaymentNotFound
	}

	cp := *payment

	return &cp, nil
}

func (f *fakePaymentStore) FindPaymentByCommerceOrder(
	_ context.Context,
	commerceOrderID uuid.UUID,
) (*model.Payment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	payment, ok := f.byCO[commerceOrderID]
	if !ok {
		return nil, repository.ErrPaymentNotFound
	}

	cp := *payment

	return &cp, nil
}

func (f *fakePaymentStore) FindPaymentByProviderOrder(
	_ context.Context,
	providerOrderID string,
) (*model.Payment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	payment, ok := f.byPO[providerOrderID]
	if !ok {
		return nil, repository.ErrPaymentNotFound
	}

	cp := *payment

	return &cp, nil
}

func (f *fakePaymentStore) FindPaymentByProviderPayment(
	_ context.Context,
	providerPaymentID string,
) (*model.Payment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	payment, ok := f.byPP[providerPaymentID]
	if !ok {
		return nil, repository.ErrPaymentNotFound
	}

	cp := *payment

	return &cp, nil
}

func (f *fakePaymentStore) UpdateProviderOrder(
	_ context.Context,
	id uuid.UUID,
	providerOrderID string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	payment, ok := f.byID[id]
	if !ok {
		return repository.ErrPaymentNotFound
	}

	payment.ProviderOrderID = &providerOrderID
	f.byPO[providerOrderID] = payment

	return nil
}

func (f *fakePaymentStore) CapturePaymentTx(
	_ context.Context,
	id uuid.UUID,
	providerPaymentID string,
	providerSignature string,
) (*model.Payment, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	payment, ok := f.byID[id]
	if !ok {
		return nil, false, repository.ErrPaymentNotFound
	}

	if payment.Status == model.PaymentCaptured {
		cp := *payment

		return &cp, true, nil
	}

	if payment.Status != model.PaymentCreated &&
		payment.Status != model.PaymentAuthorized {
		return nil, false, repository.ErrPaymentConflict
	}

	now := time.Now()
	payment.Status = model.PaymentCaptured
	payment.ProviderPaymentID = &providerPaymentID
	payment.ProviderSignature = &providerSignature
	payment.CapturedAt = &now
	payment.UpdatedAt = now
	f.byPP[providerPaymentID] = payment

	cp := *payment

	return &cp, false, nil
}

func (f *fakePaymentStore) FailPaymentTx(
	_ context.Context,
	id uuid.UUID,
	code string,
	reason string,
) (*model.Payment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	payment, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrPaymentNotFound
	}

	if payment.Final() {
		cp := *payment

		return &cp, nil
	}

	now := time.Now()
	payment.Status = model.PaymentFailed
	payment.FailureCode = &code
	payment.FailureReason = &reason
	payment.FailedAt = &now
	payment.UpdatedAt = now

	cp := *payment

	return &cp, nil
}

func (f *fakePaymentStore) MarkPaymentRefunded(
	_ context.Context,
	id uuid.UUID,
	partial bool,
) (*model.Payment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	payment, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrPaymentNotFound
	}

	if payment.Status != model.PaymentCaptured {
		return nil, repository.ErrPaymentConflict
	}

	if partial {
		payment.Status = model.PaymentPartiallyRefunded
	} else {
		payment.Status = model.PaymentRefunded
	}

	payment.UpdatedAt = time.Now()
	cp := *payment

	return &cp, nil
}

// fakeCommerceClient serves scripted orders and records outcomes.
type fakeCommerceClient struct {
	mu      sync.Mutex
	orders  map[uuid.UUID]*commerce.Order
	err     error
	paid    []uuid.UUID
	failed  []uuid.UUID
	paidErr error
	failErr error
}

func (f *fakeCommerceClient) GetOrder(
	_ context.Context,
	orderID uuid.UUID,
) (*commerce.Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return nil, f.err
	}

	order, ok := f.orders[orderID]
	if !ok {
		return nil, commerce.ErrOrderNotFound
	}

	cp := *order

	return &cp, nil
}

func (f *fakeCommerceClient) MarkOrderPaid(
	_ context.Context,
	request commerce.MarkOrderPaidRequest,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.paidErr != nil {
		return f.paidErr
	}

	f.paid = append(f.paid, request.OrderID)

	return nil
}

func (f *fakeCommerceClient) MarkOrderFailed(
	_ context.Context,
	request commerce.MarkOrderFailedRequest,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.failErr != nil {
		return f.failErr
	}

	f.failed = append(f.failed, request.OrderID)

	return nil
}

// fakeRazorpay is a programmable razorpay.Client.
type fakeRazorpay struct {
	mu         sync.Mutex
	orderID    string
	orderErr   error
	payment    razorpay.PaymentResponse
	paymentErr error
	refund     razorpay.RefundResponse
	refundErr  error
	ordersMade int
}

func (f *fakeRazorpay) CreateOrder(
	_ context.Context,
	_ razorpay.CreateOrderRequest,
) (razorpay.CreateOrderResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.orderErr != nil {
		return razorpay.CreateOrderResponse{}, f.orderErr
	}

	f.ordersMade++

	return razorpay.CreateOrderResponse{
		ID:       f.orderID,
		Amount:   99900,
		Currency: "INR",
		Status:   "created",
	}, nil
}

func (f *fakeRazorpay) FetchPayment(
	_ context.Context,
	_ string,
) (razorpay.PaymentResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.paymentErr != nil {
		return razorpay.PaymentResponse{}, f.paymentErr
	}

	return f.payment, nil
}

func (f *fakeRazorpay) FetchOrder(
	_ context.Context,
	_ string,
) (razorpay.OrderResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return razorpay.OrderResponse{ID: f.orderID}, nil
}

func (f *fakeRazorpay) CreateRefund(
	_ context.Context,
	_ string,
	_ razorpay.RefundRequest,
) (razorpay.RefundResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.refundErr != nil {
		return razorpay.RefundResponse{}, f.refundErr
	}

	return f.refund, nil
}

type fixture struct {
	payments *PaymentService
	refunds  *RefundService
	webhooks *WebhookService

	paymentStore *fakePaymentStore
	refundStore  *fakeRefundStore
	webhookStore *fakeWebhookStore
	commerce     *fakeCommerceClient
	razorpay     *fakeRazorpay
}

func newFixture() *fixture {
	paymentStore := newFakePaymentStore()
	refundStore := newFakeRefundStore()
	webhookStore := newFakeWebhookStore()
	commerce := &fakeCommerceClient{orders: make(map[uuid.UUID]*commerce.Order)}
	razorpay := &fakeRazorpay{orderID: "order_test123"}

	payments, err := NewPaymentService(
		paymentStore,
		commerce,
		razorpay,
		"rzp_test_key",
		"test-secret",
	)
	if err != nil {
		panic(err)
	}

	refunds, err := NewRefundService(refundStore, paymentStore, razorpay)
	if err != nil {
		panic(err)
	}

	webhooks, err := NewWebhookService(
		webhookStore,
		paymentStore,
		refunds,
		commerce,
		"test-webhook-secret",
	)
	if err != nil {
		panic(err)
	}

	return &fixture{
		payments:     payments,
		refunds:      refunds,
		webhooks:     webhooks,
		paymentStore: paymentStore,
		refundStore:  refundStore,
		webhookStore: webhookStore,
		commerce:     commerce,
		razorpay:     razorpay,
	}
}

func payableOrder(id, userID uuid.UUID) *commerce.Order {
	return &commerce.Order{
		ID:            id,
		UserID:        userID,
		OrderNumber:   "EDV-20240101-ABC123",
		Status:        "pending_payment",
		Currency:      "INR",
		SubtotalCents: 99900,
		TotalCents:    99900,
	}
}

func capturedPayment(
	t *testing.T,
	fx *fixture,
	userID uuid.UUID,
	orderID uuid.UUID,
	providerOrder string,
	providerPayment string,
) *model.Payment {
	t.Helper()

	fx.razorpay.orderID = providerOrder
	fx.razorpay.payment = razorpay.PaymentResponse{
		ID:       providerPayment,
		OrderID:  providerOrder,
		Amount:   99900,
		Currency: "INR",
		Status:   "captured",
		Captured: true,
	}

	intent, err := fx.payments.CreatePayment(context.Background(), userID, orderID)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	signature := testSignature(providerOrder, providerPayment)

	verified, err := fx.payments.VerifyPayment(context.Background(), userID, VerifyInput{
		CommerceOrderID:   orderID,
		ClientOrderID:     providerOrder,
		ProviderPaymentID: providerPayment,
		Signature:         signature,
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	_ = intent

	return verified
}

func testSignature(orderID, paymentID string) string {
	return signForTest(orderID, paymentID)
}
