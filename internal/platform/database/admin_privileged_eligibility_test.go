package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestBackupSettingsAndPMCommandsRequireCurrentAdministrator(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2026, 11, 13, 19, 0, 0, 0, time.UTC)
	actor := createTestAdmin(t, store, 89116)
	member := pmMember(t, store, 89117)
	if _, err := store.QueuePMRelay(ctx, model.PMRelayInput{ActorUserID: member.ID, UserID: member.ID,
		UpdateID: 801, ChatID: -100123, SourceChatID: member.TelegramID, SourceMessageID: 1, Inbound: true}, now); err != nil {
		t.Fatal(err)
	}
	conversation, found, err := store.PMConversationByUser(ctx, member.ID)
	if err != nil || !found {
		t.Fatalf("conversation fixture = %+v, %t, %v", conversation, found, err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE telegram_pm_conversations SET topic_state='pending_review',profile_state='pending_review' WHERE id=$1`, conversation.ID); err != nil {
		t.Fatal(err)
	}
	muted := true
	for _, state := range []struct {
		name, role, status string
		blocked            int
	}{
		{"demoted", "user", "active", 0},
		{"purging", "admin", "purging", 0},
		{"blocked", "admin", "active", 1},
	} {
		t.Run(state.name, func(t *testing.T) {
			if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role=$1,account_status=$2,bot_blocked=$3 WHERE id=$4`,
				state.role, state.status, state.blocked, actor.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := store.StartAuditedBackupRun(ctx, "/managed/rejected.dump", actor.ID, now); !errors.Is(err, ErrConflict) {
				t.Fatalf("ineligible backup = %v", err)
			}
			if err := store.PutAdminSetting(ctx, "privileged-secret", "sealed", true, actor.ID, now); !errors.Is(err, ErrConflict) {
				t.Fatalf("ineligible setting = %v", err)
			}
			if _, err := store.QueuePMModeration(ctx, actor.ID, "fenced-moderation", model.PMModerationInput{
				ConversationID: conversation.ID, Muted: &muted}, 802, now); !errors.Is(err, ErrConflict) {
				t.Fatalf("ineligible moderation = %v", err)
			}
			if _, err := store.QueuePMProfileRefresh(ctx, actor.ID, "fenced-refresh", conversation.ID, 803, now); !errors.Is(err, ErrConflict) {
				t.Fatalf("ineligible profile refresh = %v", err)
			}
			if _, err := store.QueuePMTopicRepair(ctx, actor.ID, "fenced-repair", model.PMTopicRepairInput{
				ConversationID: conversation.ID, TopicID: 100, ProfileMessageID: 101}, now); !errors.Is(err, ErrConflict) {
				t.Fatalf("ineligible topic repair = %v", err)
			}
			if _, err := store.QueuePMRelay(ctx, model.PMRelayInput{ActorUserID: actor.ID, UserID: member.ID,
				UpdateID: 804, ChatID: -100123, SourceChatID: -100123, SourceMessageID: 2}, now); !errors.Is(err, ErrConflict) {
				t.Fatalf("ineligible outbound relay = %v", err)
			}
		})
	}
	for _, item := range []struct {
		query string
		want  int
	}{
		{`SELECT COUNT(*) FROM backup_runs`, 0},
		{`SELECT COUNT(*) FROM provider_operations`, 1},
		{`SELECT COUNT(*) FROM telegram_pm_updates`, 1},
		{`SELECT COUNT(*) FROM settings WHERE key='privileged-secret'`, 0},
	} {
		var count int
		if err := store.DB().QueryRowContext(ctx, item.query).Scan(&count); err != nil || count != item.want {
			t.Fatalf("rejected command retained side effects: %d, %v; want %d", count, err, item.want)
		}
	}
	var actorAudits int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE actor_user_id=$1`, actor.ID).Scan(&actorAudits); err != nil || actorAudits != 0 {
		t.Fatalf("rejected command success audits = %d, %v", actorAudits, err)
	}
	retained, _, err := store.PMConversationByUser(ctx, member.ID)
	if err != nil || retained.Muted || retained.Blocked {
		t.Fatalf("rejected moderation changed member flags: %+v, %v", retained, err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role='admin',account_status='active',bot_blocked=0 WHERE id=$1`, actor.ID); err != nil {
		t.Fatal(err)
	}
	if run, err := store.StartAuditedBackupRun(ctx, "/managed/accepted.dump", actor.ID, now); err != nil || run.ID == "" {
		t.Fatalf("restored backup = %+v, %v", run, err)
	}
	if err := store.PutAdminSetting(ctx, "privileged-secret", "sealed", true, actor.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.QueuePMModeration(ctx, actor.ID, "fenced-moderation", model.PMModerationInput{
		ConversationID: conversation.ID, Muted: &muted}, 802, now); err != nil {
		t.Fatal(err)
	}
}
