package xnode_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

type statisticsPublicationRepository struct {
	*database.Store
	onPeriods func()
}

func (r *statisticsPublicationRepository) XNodePeriods(ctx context.Context, id int64) ([]xnode.Period, error) {
	rows, err := r.Store.XNodePeriods(ctx, id)
	if err == nil && r.onPeriods != nil {
		hook := r.onPeriods
		r.onPeriods = nil
		hook()
	}
	return rows, err
}

func TestStatisticsDiscardsDashboardAfterTokenChangesDuringRead(t *testing.T) {
	f := controlFixture(t, true)
	ctx := context.Background()
	repo := &statisticsPublicationRepository{Store: f.store}
	service := publicationService(t, repo, &fixtureSettings{})
	called := false
	repo.onPeriods = func() {
		called = true
		if _, err := f.service.RevokeSubscription(ctx, stringID(f.identity)); err != nil {
			t.Fatal(err)
		}
	}
	stats, link, err := service.Statistics(ctx, stringID(f.identity))
	if !called || !errors.Is(err, xnode.ErrConflict) || link != "" || !reflect.DeepEqual(stats, model.Statistics{}) {
		t.Fatal("dashboard published a stale bearer", called, stats, link, err)
	}
}

func TestStatisticsDoesNotAttributePredecessorUsageToUnprovisionedSuccessor(t *testing.T) {
	f := controlFixture(t, true)
	ctx := context.Background()
	identity, err := f.store.XNodeIdentityByUser(ctx, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.DB().ExecContext(ctx, `UPDATE xnode_periods SET used_bytes=12345,status='closed' WHERE id=$1`, identity.CurrentPeriodID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.DB().ExecContext(ctx, `UPDATE purchases SET status='cancelled' WHERE user_id=$1`, f.user.ID); err != nil {
		t.Fatal(err)
	}
	combo, err := f.store.SaveCombo(ctx, database.ComboInput{Name: "Successor", ValidityDays: 30, TrafficLimitBytes: 98765, ResetStrategy: "MONTH_ROLLING", Active: true, SquadProductIDs: []string{f.group.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.CreatePurchase(ctx, database.PurchaseInput{UserID: f.user.ID, ComboID: combo.ID, IdempotencyKey: "statistics-successor"}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	stats, _, err := f.service.Statistics(ctx, stringID(identity))
	if err != nil || stats.UsedTrafficBytes != "0" || stats.TrafficLimitBytes != "98765" ||
		stats.LastTrafficResetAt != nil || stats.LifetimeTrafficBytes != "12345" {
		t.Fatal("historical counters appeared in the successor", stats, err)
	}
}
