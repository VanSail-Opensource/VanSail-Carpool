package database

import (
	"context"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"path/filepath"
	"testing"
)

// Both paths read the same synthetic member set; setup is outside measurement.
func BenchmarkXNodeCompileMemberReads(b *testing.B) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(b.TempDir(), "members.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	s := NewStore(db)
	ids := []string{}
	for i := range 100 {
		user, _, err := s.UpsertTelegramUser(ctx, model.TelegramProfile{ID: int64(900000 + i), FirstName: fmt.Sprintf("Synthetic member %d", i)}, false)
		if err != nil {
			b.Fatal(err)
		}
		ids = append(ids, user.ID)
	}
	b.Run("Individual", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			for _, id := range ids {
				if _, err = s.UserByID(ctx, id); err != nil {
					b.Fatal(err)
				}
				if _, err = s.ListPurchases(ctx, id); err != nil {
					b.Fatal(err)
				}
				if _, err = s.UserDisabledSquads(ctx, id); err != nil {
					b.Fatal(err)
				}
				if _, err = s.XNodeUserRestricted(ctx, id); err != nil {
					b.Fatal(err)
				}
			}
		}
		b.ReportMetric(float64(4*len(ids)), "queries/op")
	})
	b.Run("Batched", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err = s.XNodeCompileMembers(ctx, ids); err != nil {
				b.Fatal(err)
			}
		}
		b.ReportMetric(6, "queries/op")
	})
}
