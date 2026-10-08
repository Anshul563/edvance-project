package event

import (
	"github.com/Anshul563/edvance-project/services/notification-service/internal/model"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/ntype"
)

// authDefaults maps identity/security events. Security types bypass
// every preference toggle downstream and always ship critical/high.
func authDefaults() map[string]Defaults {
	return map[string]Defaults{
		ntype.UserEmailVerified: {
			Title:    "Email verified",
			Body:     "Your email address is verified.",
			Channels: inAppEmail(),
			Priority: model.PriorityNormal,
		},
		ntype.AuthPasswordReset: {
			Title:    "Password reset",
			Body:     "Use the link we emailed you to choose a new password.",
			Channels: inAppEmail(),
			Priority: model.PriorityHigh,
		},
		ntype.AuthSecurityAlert: {
			Title:    "Security alert",
			Body:     "We noticed unusual activity on your account. If this was not you, reset your password now.",
			Channels: inAppEmail(),
			Priority: model.PriorityCritical,
		},
	}
}
