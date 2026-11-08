package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestPurgeRetainsLegacyEmbyCreateEvidenceIndependentlyOfParentAttempt(t *testing.T) {
	for _, parent := range []bool{false, true} {
		name := "legacy job only"
		if parent {
			name = "unattempted parent with attempted account"
		}
		t.Run(name, func(t *testing.T) {
			ctx, s := context.Background(), newTestStore(t)
			u := createTestUser(t, s, 497864)
			now := time.Now().UTC()
			if _, err := s.AdjustBalance(ctx, u.ID, 1000, "emby-purge-seed", "seed", now); err != nil {
				t.Fatal(err)
			}
			account, _, err := s.QueueEmbySetup(ctx, emby.QueueSetupInput{
				ID: "legacy-emby", UserID: u.ID, BaseUsername: u.AccountName(), SetupPriceTXBMinor: 275,
				PasswordCiphertext: "synthetic-password-cipher", PasswordContext: "synthetic-context",
			}, now)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.DB().ExecContext(ctx, `UPDATE emby_accounts SET create_attempted=1 WHERE id=$1`, account.ID); err != nil {
				t.Fatal(err)
			}
			operationID := ""
			if parent {
				op, _, err := s.CreateProviderOperation(ctx, providerops.CreateInput{
					ActorUserID: u.ID, OwnerUserID: u.ID, Kind: "emby_setup", IdempotencyKey: "old-setup",
					RequestFingerprint: "legacy-setup", SealedTarget: "synthetic-sealed-target",
					Items: []providerops.ItemInput{{Key: "emby", TargetType: "emby_user", TargetID: u.ID}},
				}, now)
				if err != nil {
					t.Fatal(err)
				}
				operationID = op.Receipt.ID
			}
			admitTestAccountCleanup(t, s, u, now)
			if err = s.FenceAccountPurge(ctx, u.ID, 1, now); err != nil {
				t.Fatal(err)
			}
			reason, err := s.AccountPurgeDependency(ctx, u.ID, 1, now)
			want := "account_purge_emby_creation_pending"
			if parent {
				want = "account_purge_provider_outcome_pending"
			}
			if err != nil || reason != want {
				t.Fatal("legacy create boundary was lost", reason, err)
			}
			if err = s.CompleteAccountPurge(ctx, u.ID, 1, now); err == nil {
				t.Fatal("uncertain Emby account permitted financial purge")
			}
			record, err := s.EmbyProvisioningByID(ctx, account.ID)
			if err != nil || !record.CreateAttempted || record.PasswordCiphertext != "synthetic-password-cipher" {
				t.Fatal("create evidence erased", err)
			}
			var jobs int
			if err = s.DB().QueryRowContext(ctx, `SELECT count(*) FROM outbox_jobs WHERE kind='emby_provision_account' AND payload::jsonb->>'accountId'=$1`, account.ID).Scan(&jobs); err != nil || jobs != 1 {
				t.Fatal("legacy continuation erased", jobs, err)
			}
			if parent {
				op, err := s.ProviderOperationByID(ctx, operationID)
				if err != nil || op.Receipt.Status != "queued" || op.Attempts != 0 {
					t.Fatal("legacy parent superseded despite account evidence", err)
				}
			}
			balance, err := s.Balance(ctx, u.ID)
			if err != nil || balance.Minor != "725" {
				t.Fatal("unknown creation was refunded or forfeited", balance.Minor, err)
			}
		})
	}
}
