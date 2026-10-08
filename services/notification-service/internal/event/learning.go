package event

import (
	"github.com/Anshul563/edvance-project/services/notification-service/internal/model"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/ntype"
)

// learningDefaults maps learner-journey events.
func learningDefaults() map[string]Defaults {
	return map[string]Defaults{
		ntype.LearningEnrolled: {
			Title:    "You're enrolled in a new course",
			Body:     "You are now enrolled in {{.courseTitle}}.",
			Channels: inAppEmail(),
			Priority: model.PriorityNormal,
		},
		ntype.LearningLessonCompleted: {
			Title:    "Lesson completed",
			Body:     "You completed {{.lessonTitle}}.",
			Channels: inAppOnly(),
			Priority: model.PriorityLow,
		},
		ntype.LearningCourseCompleted: {
			Title:    "Course completed 🎉",
			Body:     "Congratulations! You completed {{.courseTitle}}.",
			Channels: inAppEmail(),
			Priority: model.PriorityHigh,
		},
	}
}
