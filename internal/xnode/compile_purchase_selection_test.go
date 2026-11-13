package xnode

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestCompilePurchaseSelectionUsesParsedChronologyRegardlessOfInputOrder(t *testing.T) {
	t.Parallel()
	base := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	first := model.Purchase{ID: "older", Status: "active", ValidFrom: base, ValidUntil: base.Add(time.Hour), CreatedAt: base}
	for _, test := range []struct {
		name string
		next model.Purchase
	}{
		{"fractional start", model.Purchase{ID: "newer", ValidFrom: base.Add(500 * time.Millisecond), CreatedAt: base}},
		{"fractional creation", model.Purchase{ID: "newer", ValidFrom: base, CreatedAt: base.Add(time.Nanosecond)}},
		{"stable identity", model.Purchase{ID: "z-newer", ValidFrom: base, CreatedAt: base}},
	} {
		t.Run(test.name, func(t *testing.T) {
			next := test.next
			next.Status, next.ValidUntil = "activating", base.Add(time.Hour)
			future := next
			future.ID, future.ValidFrom = "future", base.Add(2*time.Second)
			expired := next
			expired.ID, expired.ValidUntil = "expired", base.Add(time.Second)
			for _, purchases := range [][]model.Purchase{{first, future, next, expired}, {expired, next, first, future}} {
				repo := &compileReads{desired: map[string]*model.Purchase{}, purchases: map[string][]model.Purchase{"owner": purchases}}
				got, err := repo.DesiredEntitlement(context.Background(), "owner", base.Add(time.Second))
				if err != nil || got == nil || got.ID != next.ID {
					t.Fatal("compile selected lexical, future or expired purchase", got, err)
				}
			}
		})
	}
}
