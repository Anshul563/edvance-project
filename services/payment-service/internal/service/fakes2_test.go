package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"sync"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/payment-service/internal/model"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/repository"
)

func signForTest(orderID, paymentID string) string {
	mac := hmac.New(sha256.New, []byte("test-secret"))
	mac.Write([]byte(orderID + "|" + paymentID))

	return hex.EncodeToString(mac.Sum(nil))
}

// fakeRefundStore is an in-memory RefundStore.
type fakeRefundStore struct {
	mu     sync.Mutex
	byID   map[uuid.UUID]*model.Refund
	byProv map[string]*model.Refund
	byPay  map[uuid.UUID][]*model.Refund
	byKey  map[string]*model.Refund
}

func newFakeRefundStore() *fakeRefundStore {
	return &fakeRefundStore{
		byID:   make(map[uuid.UUID]*model.Refund),
		byProv: make(map[string]*model.Refund),
		byPay:  make(map[uuid.UUID][]*model.Refund),
		byKey:  make(map[string]*model.Refund),
	}
}

func (f *fakeRefundStore) CreateRefund(
	_ context.Context,
	refund *model.Refund,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	refund.ID = uuid.New()
	if refund.IdempotencyKey != nil {
		key := refund.PaymentID.String() + ":" + *refund.IdempotencyKey
		if _, exists := f.byKey[key]; exists {
			return repository.ErrRefundIdempotencyKeyTaken
		}
	}

	stored := *refund
	f.byID[refund.ID] = &stored
	f.byPay[refund.PaymentID] = append(f.byPay[refund.PaymentID], &stored)
	if refund.IdempotencyKey != nil {
		f.byKey[refund.PaymentID.String()+":"+*refund.IdempotencyKey] = &stored
	}

	return nil
}

func (f *fakeRefundStore) FindRefundByPaymentIdempotencyKey(
	_ context.Context,
	paymentID uuid.UUID,
	key string,
) (*model.Refund, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	refund, ok := f.byKey[paymentID.String()+":"+key]
	if !ok {
		return nil, repository.ErrRefundNotFound
	}
	copy := *refund
	return &copy, nil
}

func (f *fakeRefundStore) FindRefundByID(
	_ context.Context,
	id uuid.UUID,
) (*model.Refund, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	refund, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrRefundNotFound
	}

	cp := *refund

	return &cp, nil
}

func (f *fakeRefundStore) FindRefundByProviderID(
	_ context.Context,
	providerRefundID string,
) (*model.Refund, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	refund, ok := f.byProv[providerRefundID]
	if !ok {
		return nil, repository.ErrRefundNotFound
	}

	cp := *refund

	return &cp, nil
}

func (f *fakeRefundStore) ListRefundsByPayment(
	_ context.Context,
	paymentID uuid.UUID,
) ([]*model.Refund, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := []*model.Refund{}

	for _, refund := range f.byPay[paymentID] {
		cp := *refund
		out = append(out, &cp)
	}

	return out, nil
}

func (f *fakeRefundStore) MarkRefundProcessed(
	_ context.Context,
	id uuid.UUID,
	providerRefundID string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	refund, ok := f.byID[id]
	if !ok {
		return repository.ErrRefundNotFound
	}

	if refund.Status != model.RefundCreated {
		return repository.ErrRefundConflict
	}

	refund.Status = model.RefundProcessed
	refund.ProviderRefundID = &providerRefundID
	f.byProv[providerRefundID] = refund

	return nil
}

func (f *fakeRefundStore) SetProviderRefundID(
	_ context.Context,
	id uuid.UUID,
	providerRefundID string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	refund, ok := f.byID[id]
	if !ok {
		return repository.ErrRefundNotFound
	}
	if refund.Status != model.RefundCreated {
		return repository.ErrRefundConflict
	}
	refund.ProviderRefundID = &providerRefundID
	f.byProv[providerRefundID] = refund
	return nil
}

func (f *fakeRefundStore) MarkRefundFailed(
	_ context.Context,
	id uuid.UUID,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	refund, ok := f.byID[id]
	if !ok {
		return repository.ErrRefundNotFound
	}

	if refund.Status != model.RefundCreated {
		return repository.ErrRefundConflict
	}

	refund.Status = model.RefundFailed

	return nil
}

// fakeWebhookStore is an in-memory WebhookStore.
type fakeWebhookStore struct {
	mu     sync.Mutex
	byID   map[uuid.UUID]*model.WebhookEvent
	byProv map[string]*model.WebhookEvent
}

func newFakeWebhookStore() *fakeWebhookStore {
	return &fakeWebhookStore{
		byID:   make(map[uuid.UUID]*model.WebhookEvent),
		byProv: make(map[string]*model.WebhookEvent),
	}
}

func keyOf(provider string, eventID *string) string {
	if eventID == nil {
		return ""
	}

	return provider + ":" + *eventID
}

func (f *fakeWebhookStore) StoreEvent(
	_ context.Context,
	event *model.WebhookEvent,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	key := keyOf(event.Provider, event.EventID)

	if key != "" {
		if _, exists := f.byProv[key]; exists {
			return repository.ErrWebhookDuplicate
		}
	}

	event.ID = uuid.New()

	stored := *event
	f.byID[event.ID] = &stored

	if key != "" {
		f.byProv[key] = &stored
	}

	return nil
}

func (f *fakeWebhookStore) FindEventByProviderID(
	_ context.Context,
	provider string,
	eventID string,
) (*model.WebhookEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	event, ok := f.byProv[provider+":"+eventID]
	if !ok {
		return nil, repository.ErrWebhookNotFound
	}

	cp := *event

	return &cp, nil
}

func (f *fakeWebhookStore) MarkEventProcessed(
	_ context.Context,
	id uuid.UUID,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	event, ok := f.byID[id]
	if !ok {
		return repository.ErrWebhookNotFound
	}

	if event.Status != model.WebhookReceived {
		return repository.ErrWebhookConflict
	}

	event.Status = model.WebhookProcessed

	return nil
}

func (f *fakeWebhookStore) MarkEventFailed(
	_ context.Context,
	id uuid.UUID,
	reason string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	event, ok := f.byID[id]
	if !ok {
		return repository.ErrWebhookNotFound
	}

	event.Status = model.WebhookFailed
	event.ErrorMessage = &reason

	return nil
}
