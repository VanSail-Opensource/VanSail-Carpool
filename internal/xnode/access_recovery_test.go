package xnode_test

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestAccessRepairResumesOnlyUnsettledPeriod(t *testing.T) {
	for _, status := range []string{"closing", "closed", "review"} {
		t.Run(status, func(t *testing.T) {
			f := controlFixture(t, true)
			ctx := context.Background()
			identity, err := f.store.XNodeIdentityByUser(ctx, f.user.ID)
			if err != nil {
				t.Fatal(err)
			}
			_, err = f.store.DB().ExecContext(ctx, `UPDATE xnode_periods SET status=?,used_bytes=12345,ended_at=? WHERE id=?`, status, time.Now().UTC().Format(time.RFC3339Nano), identity.CurrentPeriodID)
			if err != nil {
				t.Fatal(err)
			}
			err = f.service.RepairAccess(ctx, model.OutboxJob{Payload: `{"userId":"` + f.user.ID + `"}`})
			if status == "closing" && err != nil {
				t.Fatal(err)
			}
			if status != "closing" && !errors.Is(err, xnode.ErrAccessAccountingPending) {
				t.Fatalf("settled/review repair: %v", err)
			}
			period, err := f.store.XNodePeriod(ctx, identity.CurrentPeriodID)
			if err != nil || period.UsedBytes != 12345 {
				t.Fatalf("usage changed: %+v %v", period, err)
			}
			if status == "closing" && (period.Status != "open" || period.EndedAt != nil) {
				t.Fatalf("not resumed: %+v", period)
			}
			if status != "closing" && period.Status != status {
				t.Fatalf("settled state changed: %+v", period)
			}
			periods, err := f.store.XNodePeriods(ctx, identity.ProxyUserID)
			if err != nil || len(periods) != 1 {
				t.Fatalf("fresh allowance created: %d %v", len(periods), err)
			}
		})
	}
}

type changingAccessRepository struct {
	xnode.Repository
	blocked  bool
	attempts int
	change   func()
}

func (r *changingAccessRepository) ComboControlConflict(context.Context, string) (bool, error) {
	return r.blocked, nil
}
func (r *changingAccessRepository) RepairXNodeAccessPeriod(context.Context, xnode.Identity, model.Purchase, xnode.Period, time.Time) error {
	r.attempts++
	if r.change != nil {
		r.change()
	}
	return xnode.ErrConflict
}

func TestAccessRepairReloadsAfterConflictAndDefersContention(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		t.Run(map[bool]string{false: "contention", true: "cancelled entitlement"}[cancel], func(t *testing.T) {
			f := controlFixture(t, true)
			r := &changingAccessRepository{Repository: f.store}
			if cancel {
				r.change = func() {
					if _, err := f.store.DB().Exec(`UPDATE purchases SET status='cancelled' WHERE user_id=?`, f.user.ID); err != nil {
						t.Fatal(err)
					}
				}
			}
			vault, err := secret.NewVault(make([]byte, 32))
			if err != nil {
				t.Fatal(err)
			}
			public, _ := url.Parse("https://panel.example")
			s := xnode.NewService(r, &fixtureSettings{}, vault, public, nil)
			err = s.RepairAccess(context.Background(), model.OutboxJob{Payload: `{"userId":"` + f.user.ID + `"}`})
			if cancel && (err != nil || r.attempts != 1) {
				t.Fatalf("stale entitlement retried: %d %v", r.attempts, err)
			}
			if !cancel && (!errors.Is(err, outbox.ErrBusy) || r.attempts != 3) {
				t.Fatalf("unbounded contention: %d %v", r.attempts, err)
			}
		})
	}
}

func TestAccessRepairRespectsOperationOwner(t *testing.T) {
	f := controlFixture(t, true)
	r := &changingAccessRepository{Repository: f.store, blocked: true}
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	public, _ := url.Parse("https://panel.example")
	s := xnode.NewService(r, &fixtureSettings{}, vault, public, nil)
	err = s.RepairAccess(context.Background(), model.OutboxJob{Payload: `{"userId":"` + f.user.ID + `"}`})
	if !errors.Is(err, outbox.ErrBusy) || err.Error() != xnode.ErrAccessOperationPending.Error() || r.attempts != 0 {
		t.Fatalf("operation bypassed: %d %v", r.attempts, err)
	}
}
