package events

import "strings"

// EventType constants use the domain.event verb convention, adapted to
// the names already defined in notification-service/internal/ntype and
// the task contract. Only event families whose corresponding features
// actually exist in the repository are included.
//
// Publisher ownership:
//   - video.*            : video-service (media pipeline authority)
//   - content.*          : content-service (publication authority)
//   - course.*           : course-service (catalog authority)
//   - learning.*         : learning-service (enrollment/progress authority)
//   - payment.* / order.*: payment-service (payment outcome authority)
//   - live.session.*     : live-streaming-service (session lifecycle)
//
// Data payloads contain opaque identifiers and minimal fields only:
// never passwords, tokens, signed URLs, or sensitive personal data.

const (
	// Video and media processing (owned by video-service).
	VideoUploaded            = "video.uploaded"
	VideoProcessingRequested = "video.processing.requested"
	VideoProcessingCompleted = "video.processing.completed"
	VideoProcessingFailed    = "video.processing.failed"
	VideoPublished           = "video.published"
	VideoUnpublished         = "video.unpublished"
	VideoDeleted             = "video.deleted"
	VideoViewed              = "video.viewed"

	// Content publication (owned by content-service).
	ContentPublished   = "content.published"
	ContentUnpublished = "content.unpublished"

	// Courses (owned by course-service).
	CoursePublished   = "course.published"
	CourseUpdated     = "course.updated"
	CourseUnpublished = "course.unpublished"
	CourseDeleted     = "course.deleted"

	// Learning (owned by learning-service).
	EnrollmentCreated   = "enrollment.created"
	EnrollmentCancelled = "enrollment.cancelled"
	LessonCompleted     = "learning.lesson.completed"
	CourseCompleted     = "learning.course.completed"

	// Payments and orders (owned by payment-service).
	PaymentSucceeded = "payment.succeeded"
	PaymentFailed    = "payment.failed"
	PaymentRefunded  = "payment.refunded"
	OrderPaid        = "order.paid"
	OrderFailed      = "order.failed"

	// Live streaming (owned by live-streaming-service).
	LiveSessionCreated     = "live.session.created"
	LiveSessionStarted     = "live.session.started"
	LiveSessionEnded       = "live.session.ended"
	LiveRecordingCompleted = "live.recording.completed"
)

// DefaultSchemaVersion is the payload schema version emitted by all
// publishers in this implementation.
const DefaultSchemaVersion = 1

// SubjectFor maps an event type to its NATS subject. Subjects use a
// dot-separated hierarchy so a single JetStream stream can catch
// related families with a wildcard while consumers subscribe narrowly.
// Invalid (dotless) type names fall under edvance.invalid.
func SubjectFor(eventType string) string {
	if strings.Contains(eventType, ".") {
		return "edvance." + eventType
	}
	return "edvance.invalid." + eventType
}
