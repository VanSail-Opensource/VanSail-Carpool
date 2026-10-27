package database

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// The fixed path must not perform per-product reads or create compute work.
// Synthetic product IDs intentionally have no catalog row to read individually.
func BenchmarkPricingFixedCatalog(b *testing.B) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	db, err := Open(ctx, filepath.Join(b.TempDir(), "pricing-benchmark.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	s := NewStore(db)
	for _, count := range []int{1, 1000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			catalog := model.Catalog{Combos: make([]model.Combo, count)}
			for i := range catalog.Combos {
				catalog.Combos[i] = model.Combo{ID: fmt.Sprint(i), Price: model.TXBMoney(1000), PriceTXBMinor: 1000}
			}
			now := time.Now().UTC()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result, err := s.PersonalizePricingCatalog(context.Background(), "benchmark", catalog, now)
				if err != nil || len(result.Combos) != count {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestFixedCatalogDoesNotPersistAnalyticalWork(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	catalog := model.Catalog{Combos: make([]model.Combo, 1000)}
	for i := range catalog.Combos {
		catalog.Combos[i] = model.Combo{ID: fmt.Sprint(i), Price: model.TXBMoney(1000)}
	}
	if _, err := s.PersonalizePricingCatalog(ctx, "fixed-member", catalog, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pricing_personal_decisions`).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='pricing_compute'`).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
}
