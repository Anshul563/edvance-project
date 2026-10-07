package learning

import (
	"context"

	"github.com/google/uuid"
)

// Provisioner enrolls users in courses after successful payment. The
// service depends on this interface — never on HTTP details — so tests
// inject fakes and the transport (HTTP today, gRPC/events later) stays
// replaceable. Learning-service's database is never touched directly.
//
// The contract is idempotent: provisioning the same (user, course,
// source) twice must converge on one enrollment, never two.
type Provisioner interface {
	ProvisionEnrollment(
		ctx context.Context,
		userID uuid.UUID,
		courseID uuid.UUID,
		source string,
	) error
}
