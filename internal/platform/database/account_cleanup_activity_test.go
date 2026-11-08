package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestCleanupAuthenticatedPartialPurgeCancelsProofWithoutRestoringBenefits(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	u := createTestUser(t, s, 497615)
	at := time.Now().UTC().Truncate(time.Microsecond)
	admitTestAccountCleanup(t, s, u, at)
	if _, _, err := s.UpsertTelegramUser(ctx, model.TelegramProfile{ID: u.TelegramID}, false); !errors.Is(err, ErrAccountCleanupRecoveryPending) || !errors.Is(err, ErrAccountUnavailable) {
		t.Fatal(err)
	}
	valid, err := s.AccountCleanupAdmissionValid(ctx, u.ID, 1, time.Now().UTC())
	if err != nil || valid {
		t.Fatal("authenticated cancellation rolled back", valid, err)
	}
	got, err := s.UserByID(ctx, u.ID)
	if err != nil || got.AccountStatus != "purging" || got.AccountGeneration != 1 || got.RecoveryReason != "account_cleanup_pending" {
		t.Fatal("partial state was blindly restored", got, err)
	}
}

func TestVerifiedAuthenticationCancelsFirstInflightCleanupProbe(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	u := createTestUser(t, s, 497611)
	at := time.Now().UTC().Truncate(time.Microsecond)
	p := cleanupTestProbe(t, s, u, at, false)
	if _, _, err := s.UpsertTelegramUser(ctx, model.TelegramProfile{ID: u.TelegramID}, false); err != nil {
		t.Fatal(err)
	}
	result := cleanupTestComplete(t, s, p, model.AccountCleanupObservation{SendAction: "blocked", GetChat: "private"}, at.Add(time.Second))
	if result.Accepted || result.Admitted {
		t.Fatal("late reply recreated authenticated candidate", result)
	}
	var failures int
	var pending bool
	if err := s.DB().QueryRowContext(ctx, `SELECT failure_count,pending_nonce IS NOT NULL FROM account_cleanup_checks WHERE user_id=$1`, u.ID).Scan(&failures, &pending); err != nil || failures != 0 || pending {
		t.Fatal(failures, pending, err)
	}
}

func TestSignedSessionCancelsCandidateBeforeOldBlockedFlagRejection(t *testing.T) {
	for _, scope := range []string{"miniapp", "browser"} {
		t.Run(scope, func(t *testing.T) {
			ctx, s := context.Background(), newTestStore(t)
			u := createTestUser(t, s, 497612)
			at := time.Now().UTC().Truncate(time.Microsecond)
			hash := make([]byte, 32)
			if scope == "browser" {
				if err := s.CreateBrowserSession(ctx, hash, u.ID, at, at.Add(time.Hour)); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := s.ReplaceSession(ctx, hash, u.ID, at.Add(time.Hour)); err != nil {
					t.Fatal(err)
				}
			}
			p := cleanupTestProbe(t, s, u, at, false)
			if err := s.HandleTelegramBotStatus(ctx, u.TelegramID, 1, 1, true, false, at); err != nil {
				t.Fatal(err)
			}
			var got model.User
			var err error
			if scope == "browser" {
				got, _, err = s.BrowserSession(ctx, hash, at.Add(time.Second))
			} else {
				got, err = s.UserBySession(ctx, hash, at.Add(time.Second))
			}
			if err != nil || got.BotBlocked || got.AccountStatus != "active" {
				t.Fatal(got, err)
			}
			if result := cleanupTestComplete(t, s, p, model.AccountCleanupObservation{SendAction: "blocked", GetChat: "private"}, at.Add(2*time.Second)); result.Accepted {
				t.Fatal("session cancellation lost", result)
			}
		})
	}
}

func TestPositiveActionCancelsCandidateDuringBotVerificationOutage(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	u := createTestUser(t, s, 497613)
	at := time.Now().UTC().Truncate(time.Microsecond)
	p := cleanupTestProbe(t, s, u, at, false)
	if _, err := s.SetAccountCleanupBot(ctx, 0, at); err != nil {
		t.Fatal(err)
	}
	r := cleanupTestComplete(t, s, p, model.AccountCleanupObservation{SendAction: "reachable", GetChat: "unknown"}, at.Add(time.Second))
	if !r.Accepted || r.Admitted || r.Reason != "reachable" {
		t.Fatal(r)
	}
}

func TestCleanupAdminAuthenticationPreservesRoleAndSupersedesOldPurge(t *testing.T) {
	for _, configured := range []bool{false, true} {
		t.Run(map[bool]string{false: "stored", true: "configured"}[configured], func(t *testing.T) {
			ctx, s := context.Background(), newTestStore(t)
			u := createTestUser(t, s, 497614)
			role := "admin"
			if configured {
				role = "user"
				if err := s.ConfigureAccountCleanupAdmins([]int64{u.TelegramID}); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := s.DB().ExecContext(ctx, `UPDATE users SET role=$2,account_status='purging',account_generation=1,bot_blocked=1 WHERE id=$1`, u.ID, role); err != nil {
				t.Fatal(err)
			}
			if _, err := s.AdjustBalance(ctx, u.ID, 30, "admin-recovery", "fixture", time.Now()); err != nil {
				t.Fatal(err)
			}
			got, _, err := s.UpsertTelegramUser(ctx, model.TelegramProfile{ID: u.TelegramID}, false)
			if err != nil || got.Role != "admin" || got.AccountStatus != "active" || got.AccountGeneration != 2 || got.BotBlocked {
				t.Fatal(got, err)
			}
			if balance, err := s.Balance(ctx, u.ID); err != nil || balance.MinorInt64() != 30 {
				t.Fatal("admin recovery changed money", balance, err)
			}
		})
	}
}
