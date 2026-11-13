package database

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

func activityAdminActor(t *testing.T, store *Store) string {
	t.Helper()
	actor := createTestUser(t, store, 89301)
	if _, err := store.DB().ExecContext(context.Background(), `UPDATE users SET role='admin' WHERE id=$1`, actor.ID); err != nil {
		t.Fatal(err)
	}
	return actor.ID
}

func rejectActivityAudit(t *testing.T, store *Store) {
	t.Helper()
	for _, statement := range []string{
		`CREATE FUNCTION reject_activity_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'activity audit unavailable'; END $$`,
		`CREATE TRIGGER reject_activity_audit BEFORE INSERT ON audit_events FOR EACH ROW WHEN (NEW.target_type IN ('activity_game','lucky_draw') OR NEW.action='activity.settings.update') EXECUTE FUNCTION reject_activity_audit()`,
	} {
		if _, err := store.DB().ExecContext(context.Background(), statement); err != nil {
			t.Fatal(err)
		}
	}
}

func allowActivityAudit(t *testing.T, store *Store) {
	t.Helper()
	if _, err := store.DB().ExecContext(context.Background(), `DROP TRIGGER reject_activity_audit ON audit_events`); err != nil {
		t.Fatal(err)
	}
}

func TestAdminActivityGameAuditRollbackAndCommittedSnapshot(t *testing.T) {
	for _, create := range []bool{true, false} {
		t.Run(map[bool]string{true: "create", false: "edit"}[create], func(t *testing.T) {
			t.Parallel()
			ctx, store, now := context.Background(), newTestStore(t), time.Now().UTC()
			actor := activityAdminActor(t, store)
			input := activity.GameInput{Name: "Before", Icon: "coin", Enabled: true, WinChanceBPS: 5000,
				MinimumStakeMinor: 100, MaximumStakeMinor: 500, ReturnMultiplierBPS: 20000}
			var before activity.Game
			if !create {
				var err error
				before, err = store.SaveActivityGame(ctx, input, now)
				if err != nil {
					t.Fatal(err)
				}
				input.ID = before.ID
			}
			input.Name, input.WinChanceBPS = "After", 1000
			rejectActivityAudit(t, store)
			if _, err := store.SaveAdminActivityGame(ctx, actor, input, now); err == nil {
				t.Fatal("game committed without audit")
			}
			games, err := store.ListActivityGames(ctx, false)
			if err != nil || create && len(games) != 0 || !create && (len(games) != 1 || !reflect.DeepEqual(games[0], before)) {
				t.Fatal("failed game audit changed definition", games, err)
			}
			allowActivityAudit(t, store)
			game, err := store.SaveAdminActivityGame(ctx, actor, input, now)
			if err != nil {
				t.Fatal(err)
			}
			var count int
			var detail string
			if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*),MAX(detail) FROM audit_events WHERE actor_user_id=$1 AND action='activity_game.save' AND target_id=$2`, actor, game.ID).Scan(&count, &detail); err != nil || count != 1 {
				t.Fatal(count, err)
			}
			var audited activity.Game
			if err := json.Unmarshal([]byte(detail), &audited); err != nil || !reflect.DeepEqual(audited, game) {
				t.Fatal("audit differs from committed response", err)
			}
		})
	}
}

func TestAdminDrawAuditRollsBackPrizesAndRevision(t *testing.T) {
	t.Parallel()
	ctx, store, now := context.Background(), newTestStore(t), time.Now().UTC()
	actor := activityAdminActor(t, store)
	input := activity.LuckyDrawInput{Name: "Before", Kind: "instant", Enabled: true, FeeMinor: 100, ExpectedParticipation: 10,
		Prizes: []activity.PrizeInput{{Name: "No change", ProbabilityBPS: 10000, Reward: activity.Reward{Kind: activity.RewardNone}}}}
	before, err := store.SaveLuckyDraw(ctx, input, now)
	if err != nil {
		t.Fatal(err)
	}
	input = before.LuckyDrawInput
	input.Prizes = append([]activity.PrizeInput(nil), input.Prizes...)
	input.FeeMinor, input.Prizes[0].Name = 200, "New terms"
	rejectActivityAudit(t, store)
	if _, err := store.SaveAdminLuckyDraw(ctx, actor, input, now); err == nil {
		t.Fatal("draw edit committed without audit")
	}
	stored, err := store.LuckyDrawByID(ctx, before.ID)
	if err != nil || !reflect.DeepEqual(stored, before) {
		t.Fatal("failed draw audit changed definition/prizes", stored, err)
	}
	var revisions int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM activity_draw_revisions WHERE draw_id=$1`, before.ID).Scan(&revisions); err != nil || revisions != 1 {
		t.Fatal("failed edit left revision", revisions, err)
	}
	allowActivityAudit(t, store)
	draw, err := store.SaveAdminLuckyDraw(ctx, actor, input, now)
	if err != nil || draw.Revision != 2 || draw.FeeMinor != 200 || draw.Prizes[0].Name != "New terms" {
		t.Fatal(draw, err)
	}
	var count int
	var detail string
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*),MAX(detail) FROM audit_events WHERE actor_user_id=$1 AND action='lucky_draw.save' AND target_id=$2`, actor, draw.ID).Scan(&count, &detail); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	var audited activity.LuckyDraw
	if err := json.Unmarshal([]byte(detail), &audited); err != nil || !reflect.DeepEqual(audited, draw) {
		t.Fatal("audit differs from committed draw", err)
	}
}

func TestAdminRafflePublicationAuditRollsBackStateAndOutbox(t *testing.T) {
	t.Parallel()
	ctx, store, now := context.Background(), newTestStore(t), time.Now().UTC()
	actor := activityAdminActor(t, store)
	draw, err := store.SaveLuckyDraw(ctx, auditRaffleInput("audit", "audit_draw"), now)
	if err != nil {
		t.Fatal(err)
	}
	rejectActivityAudit(t, store)
	if err := store.PublishAdminRaffle(ctx, actor, draw.ID, -100123, now); err == nil {
		t.Fatal("publication committed without audit")
	}
	stored, err := store.LuckyDrawByID(ctx, draw.ID)
	if err != nil || stored.Status != "draft" || stored.GroupChatID != 0 {
		t.Fatal("failed audit published draft", stored, err)
	}
	var jobs int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='draw_telegram_publish'`).Scan(&jobs); err != nil || jobs != 0 {
		t.Fatal("failed publication left job", jobs, err)
	}
	allowActivityAudit(t, store)
	if err := store.PublishAdminRaffle(ctx, actor, draw.ID, -100123, now); err != nil {
		t.Fatal(err)
	}
	var audits int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE actor_user_id=$1 AND action='lucky_draw.publish' AND target_id=$2`, actor, draw.ID).Scan(&audits); err != nil || audits != 1 {
		t.Fatal(audits, err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='draw_telegram_publish'`).Scan(&jobs); err != nil || jobs != 1 {
		t.Fatal(jobs, err)
	}
}
