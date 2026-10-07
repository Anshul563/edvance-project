package course

import (
	"context"

	"github.com/google/uuid"
)

// Client reads limited course information from course-service. The
// service depends on this interface — never on HTTP details — so tests
// inject fakes. Course-service is never written to and its database is
// never touched.
type Client interface {
	GetCourse(ctx context.Context, courseID uuid.UUID) (*Course, error)
}
