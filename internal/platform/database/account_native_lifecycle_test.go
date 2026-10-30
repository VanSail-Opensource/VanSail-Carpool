package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestPurgeWaitsForAccountingAndRejoinRotatesPAYG(t *testing.T) {
	s, parent, _, a := paygBillingFixture(t)
	ctx, now := context.Background(), time.Now().UTC()
	u, err := s.UserByID(ctx, parent.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.HandleTelegramBotStatus(ctx, u.TelegramID, 50, 500, true, false, now); err != nil {
		t.Fatal(err)
	}
	if err = s.CompleteAccountPurge(ctx, u.ID, 1, now); !errors.Is(err, ErrConflict) {
		t.Fatal("unfinished accounting discarded", err)
	}
	if _, err = s.DB().ExecContext(ctx, `UPDATE xnode_periods SET status='closing' WHERE id=?`, a.PeriodID); err != nil {
		t.Fatal(err)
	}
	paygReportAt(t, s, a, 1, now, 128)
	var used int64
	if err = s.DB().QueryRowContext(ctx, `SELECT used_bytes FROM xnode_periods WHERE id=?`, a.PeriodID).Scan(&used); err != nil || used != 128 {
		t.Fatal("closing traffic lost", used, err)
	}
	// This fixture has no remote participant; simulate its completed flush barrier.
	if _, err = s.DB().ExecContext(ctx, `UPDATE xnode_periods SET status='closed',ended_at=? WHERE proxy_user_id IN (?,?)`, stamp(now), parent.ProxyUserID, a.ProxyUserID); err != nil {
		t.Fatal(err)
	}
	if err = s.CompleteAccountPurge(ctx, u.ID, 1, now); err != nil {
		t.Fatal(err)
	}
	if err = s.HandleTelegramBotStatus(ctx, u.TelegramID, 51, 501, false, false, now); err != nil {
		t.Fatal(err)
	}
	fresh, _, err := s.UpsertTelegramUser(ctx, model.TelegramProfile{ID: u.TelegramID}, false)
	if err != nil {
		t.Fatal(err)
	}
	i, err := s.XNodeIdentity(ctx, a.ProxyUserID)
	if err != nil || i.CredentialsCipher != "" {
		t.Fatal("old credentials retained", i, err)
	}
	i.CredentialsCipher = "new-encrypted-credentials"
	p := xnode.Period{ID: "fresh-payg", ProxyUserID: a.ProxyUserID, StartedAt: now.Add(time.Second)}
	if err = s.ReopenPurgedPAYG(ctx, i, p, u.ID, fresh.AccountGeneration, 200); err != nil {
		t.Fatal(err)
	}
	got, err := s.XNodeIdentity(ctx, a.ProxyUserID)
	if err != nil || got.AccessDisabled || got.CredentialsCipher != i.CredentialsCipher || got.CurrentPeriodID != p.ID {
		t.Fatal(got, err)
	}
	var credit int64
	if err = s.DB().QueryRowContext(ctx, `SELECT credit_bytes FROM xnode_payg_accounts WHERE proxy_user_id=?`, a.ProxyUserID).Scan(&credit); err != nil || credit != 0 {
		t.Fatal("prepaid benefit restored", credit, err)
	}
	if err = s.SaveXNodePeriod(ctx, xnode.Period{ID: a.PeriodID, ProxyUserID: a.ProxyUserID, StartedAt: now, Strategy: "PAYG", Status: "open"}); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("old period reopened", err)
	}
}
