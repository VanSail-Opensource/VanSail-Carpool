package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestFirstSyncPurgeRejoinRequiresFreshAppliedPeriod(t *testing.T) {
	for _, child := range []bool{false, true} {
		name := "main"
		if child {
			name = "payg"
		}
		t.Run(name, func(t *testing.T) {
			s, parent, _, payg := paygBillingFixture(t)
			ctx, now := context.Background(), time.Now().UTC()
			user, err := s.UserByID(ctx, parent.UserID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.db.ExecContext(ctx, `UPDATE xnode_nodes SET desired_revision=1,applied_revision=1 WHERE id='node'`); err != nil {
				t.Fatal(err)
			}
			principal := parent.ProxyUserID
			if child {
				principal = payg.ProxyUserID
			}
			old := capturedFirstSyncSubject(t, s, principal)
			if err = s.ConfirmXNodeFirstSync(ctx, "node", 1, []xnode.FirstSyncSubject{old}, now); err != nil {
				t.Fatal(err)
			}
			proven, err := s.XNodeIdentityByUser(ctx, user.ID)
			if err != nil || proven.FirstSyncedAt == nil {
				t.Fatal("initial native application was not confirmed", proven, err)
			}
			// Like the existing native lifecycle fixture, simulate completed
			// remote barriers before using the real admitted purge transaction.
			if _, err = s.db.ExecContext(ctx, `UPDATE xnode_periods SET status='closed',ended_at=$1 WHERE proxy_user_id IN($2,$3)`, stamp(now), parent.ProxyUserID, payg.ProxyUserID); err != nil {
				t.Fatal(err)
			}
			admitTestAccountCleanup(t, s, user, now)
			if err = s.CompleteAccountPurge(ctx, user.ID, 1, now); err != nil {
				t.Fatal(err)
			}
			retired, err := s.XNodeIdentityByUser(ctx, user.ID)
			if err != nil || retired.FirstSyncedAt != nil || retired.CredentialsCipher != "" {
				t.Fatal("retired credentials retained readiness", retired, err)
			}
			if err = s.HandleTelegramBotStatus(ctx, user.TelegramID, 51, 501, false, false, now); err != nil {
				t.Fatal(err)
			}
			fresh, _, err := s.UpsertTelegramUser(ctx, model.TelegramProfile{ID: user.TelegramID}, false)
			if err != nil {
				t.Fatal(err)
			}
			parent, err = s.SaveXNodeIdentity(ctx, xnode.Identity{ProxyUserID: retired.ProxyUserID, UserID: user.ID, CredentialsCipher: "fresh-main-credentials"})
			if err != nil {
				t.Fatal(err)
			}
			period := xnode.Period{ID: "fresh-main-period", ProxyUserID: parent.ProxyUserID, StartedAt: now.Add(time.Second), Strategy: "NO_RESET", Status: "open"}
			if err = s.SaveXNodePeriod(ctx, period); err != nil {
				t.Fatal(err)
			}
			childIdentity, err := s.XNodeIdentity(ctx, payg.ProxyUserID)
			if err != nil {
				t.Fatal(err)
			}
			childIdentity.CredentialsCipher = "fresh-payg-credentials"
			childPeriod := xnode.Period{ID: "fresh-payg-period", ProxyUserID: payg.ProxyUserID, StartedAt: now.Add(time.Second)}
			if err = s.ReopenPurgedPAYG(ctx, childIdentity, childPeriod, user.ID, fresh.AccountGeneration, 100); err != nil {
				t.Fatal(err)
			}
			if err = s.ConfirmXNodeFirstSync(ctx, "node", 1, []xnode.FirstSyncSubject{old}, now.Add(2*time.Second)); err != nil {
				t.Fatal(err)
			}
			unproven, err := s.XNodeIdentityByUser(ctx, user.ID)
			if err != nil || unproven.FirstSyncedAt != nil {
				t.Fatal("old principal acknowledgment proved fresh credentials", unproven, err)
			}
			current := capturedFirstSyncSubject(t, s, principal)
			if err = s.ConfirmXNodeFirstSync(ctx, "node", 1, []xnode.FirstSyncSubject{current}, now.Add(3*time.Second)); err != nil {
				t.Fatal(err)
			}
			proven, err = s.XNodeIdentityByUser(ctx, user.ID)
			if err != nil || proven.FirstSyncedAt == nil {
				t.Fatal("fresh applied period could not prove readiness", proven, err)
			}
		})
	}
}
