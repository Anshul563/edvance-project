package event

import (
	"github.com/Anshul563/edvance-project/services/notification-service/internal/model"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/ntype"
)

// courseDefaults maps catalog events. Titles and bodies interpolate
// event data ({{.courseTitle}}); missing variables fail loudly at
// render time rather than sending broken copy.
func courseDefaults() map[string]Defaults {
	return map[string]Defaults{
		ntype.CoursePublished: {
			Title:    "Course published",
			Body:     "Your course {{.courseTitle}} is now live.",
			Channels: inAppEmail(),
			Priority: model.PriorityNormal,
		},
		ntype.CourseUpdated: {
			Title:    "Course updated",
			Body:     "{{.courseTitle}} has new content.",
			Channels: inAppOnly(),
			Priority: model.PriorityLow,
		},
	}
}
