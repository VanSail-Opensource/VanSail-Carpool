package database

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestHourlyConfigurationRechecksCurrentAdministratorAtCommit(t *testing.T) {
	t.Parallel()
	for _, state := range []struct {
		name, role, status string
		blocked            int
	}{
		{"demoted", "user", "active", 0},
		{"purging", "admin", "purging", 0},
		{"blocked", "admin", "active", 1},
		{"empty", "admin", "active", 0},
		{"missing", "admin", "active", 0},
	} {
		t.Run(state.name, func(t *testing.T) {
			t.Parallel()
			ctx, store := context.Background(), newTestStore(t)
			actor := createTestAdmin(t, store, 498027)
			group := xnode.Group{ID: "hourly-authority", Name: "Hourly authority", Enabled: true}
			if err := store.SaveXNodeGroup(ctx, group); err != nil {
				t.Fatal(err)
			}
			policy, err := store.CompareAndSwapHourlyPolicy(ctx, actor.ID, "", "encrypted-initial")
			if err != nil {
				t.Fatal(err)
			}
			offer, err := store.SaveHourlyOffer(ctx, actor.ID, model.HourlyOffer{GroupID: group.ID,
				Enabled: true, HourlyPriceTXBMinor: 100, RequiredHours: 2}, 0)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role=$1,account_status=$2,bot_blocked=$3 WHERE id=$4`,
				state.role, state.status, state.blocked, actor.ID); err != nil {
				t.Fatal(err)
			}
			requestActor := actor.ID // The request still holds its original administrator snapshot.
			if state.name == "empty" {
				requestActor = ""
			} else if state.name == "missing" {
				requestActor = "missing-hourly-administrator"
			}
			if _, err := store.CompareAndSwapHourlyPolicy(ctx, requestActor, policy.Version, "encrypted-replacement"); !errors.Is(err, ErrConflict) {
				t.Fatal("ineligible request changed hourly policy", err)
			}
			next := offer
			next.HourlyPriceTXBMinor = 200
			if _, err := store.SaveHourlyOffer(ctx, requestActor, next, offer.Revision); !errors.Is(err, ErrConflict) {
				t.Fatal("ineligible request changed hourly offer", err)
			}
			currentPolicy, err := store.ReadHourlyPolicyRecord(ctx)
			if err != nil || currentPolicy != policy {
				t.Fatal("refused write changed encrypted policy", currentPolicy, err)
			}
			currentOffer, err := store.HourlyOffer(ctx, group.ID)
			if err != nil || !reflect.DeepEqual(currentOffer, offer) {
				t.Fatal("refused write changed offer or revision", currentOffer, err)
			}
			count := func() int {
				t.Helper()
				var audits int
				if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events
					WHERE actor_user_id=$1 AND ((action='xnode.hourly_policy.update' AND target_id=$2)
					OR (action='xnode.hourly_offer.update' AND target_id=$3))`,
					actor.ID, HourlyPolicySettingKey, group.ID).Scan(&audits); err != nil {
					t.Fatal(err)
				}
				return audits
			}
			if got := count(); got != 2 {
				t.Fatal("refused writes left success audits", got)
			}
			if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role='admin',account_status='active',bot_blocked=0 WHERE id=$1`, actor.ID); err != nil {
				t.Fatal(err)
			}
			updatedPolicy, err := store.CompareAndSwapHourlyPolicy(ctx, actor.ID, policy.Version, "encrypted-replacement")
			if err != nil || updatedPolicy.Version == policy.Version {
				t.Fatal("restored administrator could not save policy", err)
			}
			updatedOffer, err := store.SaveHourlyOffer(ctx, actor.ID, next, offer.Revision)
			if err != nil || updatedOffer.Revision != offer.Revision+1 || updatedOffer.HourlyPriceTXBMinor != 200 {
				t.Fatal("restored administrator could not save offer", updatedOffer, err)
			}
			currentPolicy, err = store.ReadHourlyPolicyRecord(ctx)
			if err != nil || currentPolicy != updatedPolicy {
				t.Fatal("policy response differs from committed snapshot", err)
			}
			currentOffer, err = store.HourlyOffer(ctx, group.ID)
			if err != nil || !reflect.DeepEqual(currentOffer, updatedOffer) || count() != 4 {
				t.Fatal("offer response or audit differs from committed snapshot", currentOffer, err)
			}
		})
	}
}
