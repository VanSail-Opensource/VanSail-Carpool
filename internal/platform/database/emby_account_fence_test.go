package database

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
)

func fencedEmbySetup(user string, telegram int64) emby.QueueSetupInput {
	return emby.QueueSetupInput{ID: "fenced-emby", UserID: user, BaseUsername: fmt.Sprint(telegram),
		SetupPriceTXBMinor: 275, PasswordCiphertext: "synthetic-encrypted", PasswordContext: "synthetic-context"}
}

func TestLegacyEmbyNewSetupRejectsClosedOwnerWithoutDebit(t *testing.T) {
	t.Parallel()
	ctx, s := context.Background(), newTestStore(t)
	u := createTestUser(t, s, 497691)
	at := time.Now().UTC()
	if _, err := s.AdjustBalance(ctx, u.ID, 1000, "emby-fence-seed", "seed", at); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().ExecContext(ctx, `UPDATE users SET account_status='purging',account_generation=1 WHERE id=$1`, u.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.QueueEmbySetup(ctx, fencedEmbySetup(u.ID, u.TelegramID), at); !errors.Is(err, ErrAccountUnavailable) {
		t.Fatal("legacy setup crossed lifecycle fence", err)
	}
	assertBalance(t, s, u.ID, "1000")
	var count int
	if err := s.DB().QueryRowContext(ctx, `SELECT count(*) FROM emby_accounts WHERE user_id=$1`, u.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("closed owner acquired a new account", count, err)
	}
}

func TestLegacyEmbyFinancialReplayDoesNotQueueAfterOwnerClosure(t *testing.T) {
	t.Parallel()
	ctx, s := context.Background(), newTestStore(t)
	u := createTestUser(t, s, 497692)
	at := time.Now().UTC()
	if _, err := s.AdjustBalance(ctx, u.ID, 1000, "emby-replay-seed", "seed", at); err != nil {
		t.Fatal(err)
	}
	input := fencedEmbySetup(u.ID, u.TelegramID)
	account, _, err := s.QueueEmbySetup(ctx, input, at)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB().ExecContext(ctx, `UPDATE users SET account_status='purging',account_generation=1 WHERE id=$1`, u.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB().ExecContext(ctx, `DELETE FROM outbox_jobs WHERE kind=$1`, emby.ProvisionOutboxKind); err != nil {
		t.Fatal(err)
	}
	got, created, err := s.QueueEmbySetup(ctx, input, at)
	if err != nil || created || got.ID != account.ID {
		t.Fatal("exact paid replay stopped being read-only", created, err)
	}
	if _, err = s.RetryEmbyProvisioning(ctx, account.ID, at); !errors.Is(err, ErrAccountUnavailable) {
		t.Fatal("legacy retry crossed lifecycle fence", err)
	}
	assertBalance(t, s, u.ID, "725")
	var count int
	if err = s.DB().QueryRowContext(ctx, `SELECT count(*) FROM outbox_jobs WHERE kind=$1`, emby.ProvisionOutboxKind).Scan(&count); err != nil || count != 0 {
		t.Fatal("read replay/retry recreated native work", count, err)
	}
}
