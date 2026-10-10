package commerce

import (
	"context"

	"github.com/google/uuid"
)

// Client talks to commerce-service for order reads and payment
// outcomes. The service depends on this interface — never on HTTP
// details — so tests inject fakes. Commerce-service's database is
// never touched. Authentication uses the internal service token,
// never user JWTs.
type Client interface {
	GetOrder(ctx context.Context, orderID uuid.UUID) (*Order, error)
	MarkOrderPaid(ctx context.Context, request MarkOrderPaidRequest) error
	MarkOrderFailed(ctx context.Context, request MarkOrderFailedRequest) error
	MarkOrderRefunded(ctx context.Context, request MarkOrderRefundedRequest) error
}
