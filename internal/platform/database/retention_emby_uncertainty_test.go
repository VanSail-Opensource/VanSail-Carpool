package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestMaintenanceUsesLegacyEmbyAttemptEvidenceBeforeRefund(t *testing.T) {
	for i, c := range []struct {
		name      string
		attempted int
		remote    string
		retain    bool
	}{
		{"unknown create", 1, "", true},
		{"recorded remote", 0, "synthetic-remote", true},
		{"never attempted", 0, "", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			ctx, s := context.Background(), newTestStore(t)
			u := createTestUser(t, s, int64(497685+i))
			now := time.Now().UTC()
			old := now.Add(-40 * 24 * time.Hour)
			if _, err := s.AdjustBalance(ctx, u.ID, 1000, "retention-emby-seed", "seed", old); err != nil {
				t.Fatal(err)
			}
			account, _, err := s.QueueEmbySetup(ctx, emby.QueueSetupInput{
				ID: "legacy-emby", UserID: u.ID, BaseUsername: fmt.Sprint(u.TelegramID), SetupPriceTXBMinor: 275,
				PasswordCiphertext: "synthetic-encrypted-password", PasswordContext: "synthetic-context",
			}, old)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.DB().ExecContext(ctx, `UPDATE emby_accounts SET create_attempted=$2,remote_user_id=NULLIF($3,'') WHERE id=$1`, account.ID, c.attempted, c.remote); err != nil {
				t.Fatal(err)
			}
			// Historical setup/provisioning used different jobs: this parent still
			// has zero attempts even when the account crossed its create boundary.
			op, _, err := s.CreateProviderOperation(ctx, providerops.CreateInput{
				ActorUserID: u.ID, OwnerUserID: u.ID, Kind: "emby_setup", IdempotencyKey: "legacy-setup",
				RequestFingerprint: "retention-legacy-setup", SealedTarget: "synthetic-encrypted-setup",
				Items: []providerops.ItemInput{{Key: "emby", TargetType: "emby_user", TargetID: u.ID}},
			}, old)
			if err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if _, err = s.CompactAndPrune(ctx, now.Add(-7*24*time.Hour), now.Add(-24*time.Hour), now); err != nil {
					t.Fatal(err)
				}
				record, err := s.EmbyProvisioningByID(ctx, account.ID)
				if err != nil {
					t.Fatal(err)
				}
				balance, err := s.Balance(ctx, u.ID)
				if err != nil {
					t.Fatal(err)
				}
				receipt, receiptErr := s.ProviderOperationByID(ctx, op.Receipt.ID)
				if c.retain {
					if record.RefundedAt != nil || record.PasswordCiphertext != "synthetic-encrypted-password" || balance.Minor != "725" || receiptErr != nil {
						t.Fatal("legacy attempt evidence was erased/refunded", balance.Minor, receiptErr)
					}
					var retained bool
					if err = s.DB().QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM outbox_jobs WHERE payload::jsonb->>'operationId'=$1 AND payload::jsonb->>'sealedTarget'='synthetic-encrypted-setup')`, op.Receipt.ID).Scan(&retained); err != nil || !retained {
						t.Fatal("legacy setup lost its encrypted continuation", retained, err)
					}
				} else {
					if record.RefundedAt == nil || record.PasswordCiphertext != "" || balance.Minor != "1000" || receiptErr != nil || receipt.Receipt.Status != "failed" {
						t.Fatal("never-started setup did not refund once and preserve its failed receipt", balance.Minor, receipt, receiptErr)
					}
					var retained bool
					if err = s.DB().QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM outbox_jobs WHERE payload::jsonb->>'operationId'=$1)`, op.Receipt.ID).Scan(&retained); err != nil || retained {
						t.Fatal("settled setup retained encrypted continuation", retained, err)
					}
				}
			}
		})
	}
}
