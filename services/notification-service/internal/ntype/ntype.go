// Package ntype centralizes notification type strings so handlers,
// services, and tests share one definition instead of scattering raw
// strings.
package ntype

const (
	UserEmailVerified = "user.email_verified"
	AuthPasswordReset = "auth.password_reset"
	AuthSecurityAlert = "auth.security_alert"

	CoursePublished = "course.published"
	CourseUpdated   = "course.updated"

	LearningEnrolled        = "learning.enrolled"
	LearningLessonCompleted = "learning.lesson_completed"
	LearningCourseCompleted = "learning.course_completed"

	PaymentCreated  = "payment.created"
	PaymentCaptured = "payment.captured"
	PaymentFailed   = "payment.failed"
	PaymentRefunded = "payment.refunded"

	OrderPaid   = "order.paid"
	OrderFailed = "order.failed"

	VideoProcessingCompleted = "video.processing.completed"
	VideoProcessingFailed    = "video.processing.failed"
)

// SecurityTypes bypass user preferences entirely: password resets,
// suspicious logins, and security alerts must always remain deliverable.
var SecurityTypes = map[string]bool{
	AuthPasswordReset: true,
	AuthSecurityAlert: true,
}

// IsSecurity reports whether a type is mandatory-delivery.
func IsSecurity(notificationType string) bool {
	return SecurityTypes[notificationType]
}
