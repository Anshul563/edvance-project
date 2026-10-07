package repository

import (
	"testing"

	"github.com/Anshul563/edvance-project/services/commerce-service/internal/model"
)

func TestCanTransitionTable(t *testing.T) {
	legal := map[model.OrderStatus][]model.OrderStatus{
		model.OrderPendingPayment: {model.OrderPaid, model.OrderFailed, model.OrderCancelled},
		model.OrderPaid:           {model.OrderRefunded, model.OrderPartiallyRefunded},
	}

	all := []model.OrderStatus{
		model.OrderPendingPayment,
		model.OrderPaid,
		model.OrderFailed,
		model.OrderCancelled,
		model.OrderRefunded,
		model.OrderPartiallyRefunded,
	}

	for _, from := range all {
		for _, to := range all {
			want := false

			for _, ok := range legal[from] {
				if ok == to {
					want = true
				}
			}

			if got := CanTransition(from, to); got != want {
				t.Fatalf("CanTransition(%s, %s) = %v, want %v", from, to, got, want)
			}
		}
	}
}
