package event

import (
	"github.com/Anshul563/edvance-project/services/notification-service/internal/model"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/ntype"
)

// videoDefaults maps media-pipeline events. Failures stay in-app only:
// there is nothing actionable by email that the in-app note lacks.
func videoDefaults() map[string]Defaults {
	return map[string]Defaults{
		ntype.VideoProcessingCompleted: {
			Title:    "Video processing completed",
			Body:     "Your video {{.videoTitle}} is ready to publish.",
			Channels: inAppOnly(),
			Priority: model.PriorityNormal,
		},
		ntype.VideoProcessingFailed: {
			Title:    "Video processing failed",
			Body:     "Your video {{.videoTitle}} could not be processed.",
			Channels: inAppOnly(),
			Priority: model.PriorityHigh,
		},
	}
}
