package database

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func pendingPMContent(t *testing.T) (*Store, *secret.Vault, model.OperationReceipt, time.Time) {
	t.Helper()
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	store.ConfigurePMContentVault(vault)
	member := pmMember(t, store, 39120)
	admin := createTestUser(t, store, 39121)
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=?`, admin.ID); err != nil {
		t.Fatal(err)
	}
	input := model.PMRelayInput{ActorUserID: admin.ID, UserID: member.ID, UpdateID: 100, ChatID: -100123, SourceChatID: -100123, SourceMessageID: 45, Content: []byte(`{"mode":"text","text":"private pending message","footer":"By @casey_ops"}`)}
	receipt, err := store.QueuePMRelay(ctx, input, now)
	if err != nil {
		t.Fatal(err)
	}
	if duplicate, err := store.QueuePMRelay(ctx, input, now); err != nil || duplicate != nil {
		t.Fatalf("duplicate=%+v err=%v", duplicate, err)
	}
	return store, vault, *receipt, now
}

func TestPendingPMContentEncryptedBoundAndRemovedOnSettlement(t *testing.T) {
	store, vault, receipt, now := pendingPMContent(t)
	ctx := context.Background()
	var ciphertext, payload string
	if err := store.DB().QueryRowContext(ctx, `SELECT encrypted_payload FROM telegram_pm_payloads WHERE operation_id=?`, receipt.ID).Scan(&ciphertext); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(ciphertext, "private pending message") {
		t.Fatal("plaintext stored")
	}
	if _, err := vault.Decrypt("telegram_pm:wrong-operation", ciphertext); err == nil {
		t.Fatal("ciphertext not bound to operation")
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT payload FROM outbox_jobs WHERE kind='provider_operation'`).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(payload, "private pending message") || strings.Contains(payload, "casey_ops") {
		t.Fatal("plaintext escaped into outbox")
	}
	data, found, err := store.PMRelayContent(ctx, receipt.ID, now)
	if err != nil || !found || !strings.Contains(string(data), "private pending message") {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if _, err := store.BeginProviderOperationAttempt(ctx, receipt.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompleteProviderOperation(ctx, receipt.ID, providerops.Completion{Status: providerops.StatusSucceeded}, now); err != nil {
		t.Fatal(err)
	}
	if _, found, err := store.PMRelayContent(ctx, receipt.ID, now); !found || !errors.Is(err, model.ErrPMContentExpired) {
		t.Fatal("settled content can downgrade to a legacy unattributed copy")
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT encrypted_payload FROM telegram_pm_payloads WHERE operation_id=?`, receipt.ID).Scan(&ciphertext); err != nil || ciphertext != "" {
		t.Fatal("settled ciphertext retained")
	}
}

func TestPMContentExpiryLeavesExplicitMarkerForUnsentRepair(t *testing.T) {
	store, _, receipt, now := pendingPMContent(t)
	ctx := context.Background()
	if _, err := store.BeginProviderOperationAttempt(ctx, receipt.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompleteProviderOperation(ctx, receipt.ID, providerops.Completion{Status: providerops.StatusPendingReview, ErrorCode: "PM_TOPIC_UNCERTAIN"}, now); err != nil {
		t.Fatal(err)
	}
	if _, found, err := store.PMRelayContent(ctx, receipt.ID, now); err != nil || !found {
		t.Fatal("unsent repair payload removed")
	}
	expires := now.Add(24 * time.Hour)
	if _, found, err := store.PMRelayContent(ctx, receipt.ID, expires); !found || !errors.Is(err, model.ErrPMContentExpired) {
		t.Fatal("expired new content became legacy")
	}
	if _, err := store.CompactAndPrune(ctx, expires.Add(-7*24*time.Hour), expires.Add(-24*time.Hour), expires); err != nil {
		t.Fatal(err)
	}
	var ciphertext string
	if err := store.DB().QueryRowContext(ctx, `SELECT encrypted_payload FROM telegram_pm_payloads WHERE operation_id=?`, receipt.ID).Scan(&ciphertext); err != nil || ciphertext != "" {
		t.Fatal("abandoned ciphertext not erased")
	}
	if _, found, err := store.PMRelayContent(ctx, receipt.ID, expires); !found || !errors.Is(err, model.ErrPMContentExpired) {
		t.Fatal("expiry tombstone lost")
	}
}
