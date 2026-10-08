package event

import (
	"github.com/Anshul563/edvance-project/services/notification-service/internal/model"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/ntype"
)

// paymentDefaults maps money events. Amounts interpolate as preformatted
// strings ({{.amount}}); this service never formats currency itself.
func paymentDefaults() map[string]Defaults {
	return map[string]Defaults{
		ntype.PaymentCreated: {
			Title:    "Payment started",
			Body:     "Complete your payment of {{.amount}} to get access.",
			Channels: inAppOnly(),
			Priority: model.PriorityNormal,
		},
		ntype.PaymentCaptured: {
			Title:    "Payment successful",
			Body:     "Your payment for {{.courseTitle}} was successful.",
			Channels: inAppEmail(),
			Priority: model.PriorityNormal,
		},
		ntype.PaymentFailed: {
			Title:    "Payment failed",
			Body:     "Your payment could not be completed.",
			Channels: inAppEmail(),
			Priority: model.PriorityHigh,
		},
		ntype.PaymentRefunded: {
			Title:    "Refund issued",
			Body:     "A refund of {{.amount}} was issued to your account.",
			Channels: inAppEmail(),
			Priority: model.PriorityNormal,
		},
		ntype.OrderPaid: {
			Title:    "Order confirmed",
			Body:     "Your order {{.orderNumber}} is confirmed.",
			Channels: inAppEmail(),
			Priority: model.PriorityNormal,
		},
		ntype.OrderFailed: {
			Title:    "Order failed",
			Body:     "Your order could not be completed.",
			Channels: inAppEmail(),
			Priority: model.PriorityHigh,
		},
	}
}
