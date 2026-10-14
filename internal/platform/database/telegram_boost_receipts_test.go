package database

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestBoostAppreciationConcurrentAndChangedUpdatesEnqueueOnce(t *testing.T) {
	t.Parallel()
	store, ctx := newTestStore(t), context.Background()
	now := time.Now().UTC()
	item := model.TelegramBoostAppreciation{ChatID: -100123, BoostID: "boost-1", Username: "mira"}
	var group sync.WaitGroup
	for range 8 {
		group.Go(func() {
			if _, err := store.EnqueueBoostAppreciation(ctx, item, now); err != nil {
				t.Error(err)
			}
		})
	}
	group.Wait()
	item.Name, item.Username = "changed name", "changed_username"
	if added, err := store.EnqueueBoostAppreciation(ctx, item, now.Add(time.Hour)); added || err != nil {
		t.Fatalf("changed boost = %v, %v", added, err)
	}
	var count int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='telegram_boost_appreciation'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("jobs = %d, %v", count, err)
	}
	item.ChatID--
	if added, err := store.EnqueueBoostAppreciation(ctx, item, now); !added || err != nil {
		t.Fatalf("another group = %v, %v", added, err)
	}
}
