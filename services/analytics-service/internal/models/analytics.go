package models

import "time"

type Event struct {
	EventID       string         `json:"event_id"`
	EventType     string         `json:"event_type"`
	EventVersion  int            `json:"event_version"`
	OccurredAt    time.Time      `json:"occurred_at"`
	ReceivedAt    time.Time      `json:"received_at,omitempty"`
	ActorID       string         `json:"actor_id,omitempty"`
	AnonymousID   string         `json:"anonymous_id,omitempty"`
	SessionID     string         `json:"session_id,omitempty"`
	Source        string         `json:"source_service"`
	EntityType    string         `json:"entity_type"`
	EntityID      string         `json:"entity_id"`
	Properties    map[string]any `json:"properties"`
	SchemaVersion string         `json:"schema_version"`
}

type BatchEventRequest struct {
	Events []Event `json:"events"`
}

type BatchIngestResult struct {
	Accepted   int `json:"accepted"`
	Rejected   int `json:"rejected"`
	Duplicates int `json:"duplicates"`
}

type Overview struct {
	PeriodStart string `json:"period_start"`
	PeriodEnd   string `json:"period_end"`
	Events      int    `json:"events"`
	UniqueUsers int    `json:"unique_users"`
}

type CreatorOverview struct {
	CreatorID         string `json:"creator_id"`
	ContentViews      int    `json:"content_views"`
	CourseEnrollments int    `json:"course_enrollments"`
	CourseCompletions int    `json:"course_completions"`
	FollowersGained   int    `json:"followers_gained"`
	WatchTimeSeconds  int64  `json:"watch_time_seconds"`
	RevenueCents      int64  `json:"revenue_cents"`
}

type CreatorContentReport struct {
	CourseID    string `json:"course_id"`
	Views       int    `json:"views"`
	Enrollments int    `json:"enrollments"`
	Completions int    `json:"completions"`
}

type CourseOverview struct {
	CourseID         string  `json:"course_id"`
	Views            int     `json:"views"`
	Enrollments      int     `json:"enrollments"`
	Completions      int     `json:"completions"`
	CompletionRate   float64 `json:"completion_rate"`
	WatchTimeSeconds int64   `json:"watch_time_seconds"`
}

type PlatformOverview struct {
	ReportingDate     string `json:"reporting_date"`
	ActiveUsers       int    `json:"active_users"`
	NewEnrollments    int    `json:"new_enrollments"`
	CourseCompletions int    `json:"course_completions"`
	GrossRevenueCents int64  `json:"gross_revenue_cents"`
	NetRevenueCents   int64  `json:"net_revenue_cents"`
}

const (
	EventCourseViewed       = "course.viewed"
	EventCourseEnrolled     = "course.enrolled"
	EventCourseCompleted    = "course.completed"
	EventLessonStarted      = "lesson.started"
	EventLessonCompleted    = "lesson.completed"
	EventVideoViewed        = "video.viewed"
	EventVideoWatchProgress = "video.watch_progress"
	EventVideoCompleted     = "video.completed"
	EventCreatorFollowed    = "creator.followed"
	EventContentLiked       = "content.liked"
	EventContentSaved       = "content.saved"
	EventSearchPerformed    = "search.performed"
	EventCheckoutStarted    = "checkout.started"
	EventPurchaseCompleted  = "purchase.completed"
	EventPaymentFailed      = "payment.failed"
	EventLiveSessionStarted = "live.session.started"
	EventLiveSessionEnded   = "live.session.ended"
)

var SupportedEventTypes = map[string]struct{}{
	EventCourseViewed:       {},
	EventCourseEnrolled:     {},
	EventCourseCompleted:    {},
	EventLessonStarted:      {},
	EventLessonCompleted:    {},
	EventVideoViewed:        {},
	EventVideoWatchProgress: {},
	EventVideoCompleted:     {},
	EventCreatorFollowed:    {},
	EventContentLiked:       {},
	EventContentSaved:       {},
	EventSearchPerformed:    {},
	EventCheckoutStarted:    {},
	EventPurchaseCompleted:  {},
	EventPaymentFailed:      {},
	EventLiveSessionStarted: {},
	EventLiveSessionEnded:   {},
}
