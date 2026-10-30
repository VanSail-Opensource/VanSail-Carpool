package database

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestBlockedAccountClosesAnyBalanceAndCanRejoinFresh(t *testing.T) {
	for _, amount := range []int64{0, 750, -25, math.MinInt64} {
		t.Run(fmt.Sprint(amount), func(t *testing.T) {
			ctx, s := context.Background(), newTestStore(t)
			u := createTestUser(t, s, 49011)
			now := time.Now().UTC()
			if amount != 0 {
				if _, err := s.AdjustBalance(ctx, u.ID, amount, "original-balance", "fixture", now); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.ReplaceSession(ctx, make([]byte, 32), u.ID, now.Add(time.Hour)); err != nil {
				t.Fatal(err)
			}
			if err := s.HandleTelegramBotStatus(ctx, u.TelegramID, 10, 100, true, false, now); err != nil {
				t.Fatal(err)
			}
			if _, err := s.UserBySession(ctx, make([]byte, 32), now); !errors.Is(err, ErrNotFound) {
				t.Fatal("session survived", err)
			}
			if _, _, err := s.UpsertTelegramUser(ctx, model.TelegramProfile{ID: u.TelegramID}, false); !errors.Is(err, ErrAccountUnavailable) {
				t.Fatal("blocked authentication allowed", err)
			}
			if err := s.CompleteAccountPurge(ctx, u.ID, 1, now.Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			archived, err := s.UserByID(ctx, u.ID)
			if err != nil || archived.AccountStatus != "purged" || archived.TelegramFirstName != "" || archived.TelegramUsername != "" {
				t.Fatal(archived, err)
			}
			if b, err := s.Balance(ctx, u.ID); err != nil || b.MinorInt64() != 0 {
				t.Fatal(b, err)
			}
			if amount != 0 {
				var original int64
				if err := s.DB().QueryRowContext(ctx, `SELECT delta_txb_minor FROM ledger_entries WHERE kind='admin_adjustment' AND reference_id='original-balance'`).Scan(&original); err != nil || original != amount {
					t.Fatal("original ledger changed", original, err)
				}
			}
			if err := s.HandleTelegramBotStatus(ctx, u.TelegramID, 11, 101, false, false, now.Add(2*time.Second)); err != nil {
				t.Fatal(err)
			}
			fresh, _, err := s.UpsertTelegramUser(ctx, model.TelegramProfile{ID: u.TelegramID, FirstName: "Returning"}, false)
			if err != nil || fresh.AccountStatus != "active" || fresh.AccountGeneration != 1 || fresh.OnboardingState != "intro" || fresh.PolicyAcceptedAt != nil {
				t.Fatal(fresh, err)
			}
			if err := s.CompleteAccountPurge(ctx, u.ID, 1, now.Add(3*time.Second)); err != nil {
				t.Fatal(err)
			}
			fresh, err = s.UserByID(ctx, u.ID)
			if err != nil || fresh.TelegramFirstName != "Returning" {
				t.Fatal("old job erased fresh lifecycle", fresh, err)
			}
		})
	}
}

func TestBotStatusOrderingAndAdministratorExclusion(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	u := createTestUser(t, s, 49012)
	now := time.Now().UTC()
	for _, e := range []struct {
		id, at  int64
		blocked bool
	}{{10, 200, true}, {1000, 199, false}, {10, 200, true}, {1, 201, false}} {
		if err := s.HandleTelegramBotStatus(ctx, u.TelegramID, e.id, e.at, e.blocked, false, now); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.UserByID(ctx, u.ID)
	if err != nil || got.BotBlocked || got.AccountGeneration != 1 || got.AccountStatus != "purging" {
		t.Fatal(got, err)
	}
	admin := createTestUser(t, s, 49013)
	if _, err := s.DB().ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=?`, admin.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.HandleTelegramBotStatus(ctx, admin.TelegramID, 20, 300, true, false, now); err != nil {
		t.Fatal(err)
	}
	got, err = s.UserByID(ctx, admin.ID)
	if err != nil || got.AccountStatus != "active" || got.BotBlocked {
		t.Fatal("administrator purged", got, err)
	}
}

func TestPurgedAccountPaymentGoesToReviewAfterFreshRegistration(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	u := createTestUser(t, s, 49014)
	order := createTestPaymentOrder(t, s, u.ID, "ezpay", 500, time.Now().UTC())
	now := time.Now().UTC().Add(time.Second)
	if err := s.HandleTelegramBotStatus(ctx, u.TelegramID, 30, 300, true, false, now); err != nil {
		t.Fatal(err)
	}
	if err := s.CompleteAccountPurge(ctx, u.ID, 1, now); err != nil {
		t.Fatal(err)
	}
	if err := s.HandleTelegramBotStatus(ctx, u.TelegramID, 31, 301, false, false, now); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.UpsertTelegramUser(ctx, model.TelegramProfile{ID: u.TelegramID}, false); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, changed, err := s.SettlePayment(ctx, "ezpay", "late-payment", "proof", order.ID, "late-trade", "", now); err != nil || changed {
			t.Fatal(changed, err)
		}
	}
	if balance, err := s.Balance(ctx, u.ID); err != nil || balance.MinorInt64() != 0 {
		t.Fatal("old payment funded fresh account", balance, err)
	}
	var reviews int
	if err := s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE action='payment.purged_account_review' AND target_id=?`, order.ID).Scan(&reviews); err != nil || reviews != 1 {
		t.Fatal(reviews, err)
	}
}
