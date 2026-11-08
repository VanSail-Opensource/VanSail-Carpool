package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestHourlyPolicyInitialCASHasOneWinnerAndNoPlaintextAudit(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	actor := createTestUser(t, store, 498021)
	initial, err := store.ReadHourlyPolicyRecord(ctx)
	if err != nil || initial.Version != "" || initial.Ciphertext != "" {
		t.Fatal("missing policy was not absent", err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var workers sync.WaitGroup
	for i := range 2 {
		workers.Go(func() {
			<-start
			_, err := store.CompareAndSwapHourlyPolicy(ctx, actor.ID, "", fmt.Sprintf("encrypted-policy-%d", i))
			results <- err
		})
	}
	close(start)
	workers.Wait()
	close(results)
	winners, conflicts := 0, 0
	for err := range results {
		switch {
		case err == nil:
			winners++
		case errors.Is(err, ErrConflict):
			conflicts++
		default:
			t.Fatal(err)
		}
	}
	if winners != 1 || conflicts != 1 {
		t.Fatal("first setup overwrote a concurrent draft", winners, conflicts)
	}
	stored, err := store.ReadHourlyPolicyRecord(ctx)
	if err != nil || stored.Version != hourlyPolicyFingerprint(stored.Ciphertext) {
		t.Fatal("policy version was not the ciphertext fingerprint", err)
	}
	if _, err = store.CompareAndSwapHourlyPolicy(ctx, actor.ID, "", "stale-replacement"); !errors.Is(err, ErrConflict) {
		t.Fatal("stale absent version overwrote policy", err)
	}
	var count int
	var details string
	err = store.DB().QueryRowContext(ctx, `SELECT COUNT(*),MAX(detail) FROM audit_events WHERE action='xnode.hourly_policy.update'`).Scan(&count, &details)
	if err != nil || count != 1 || strings.Contains(details, "encrypted-policy-") || !strings.Contains(details, stored.Version) {
		t.Fatal("policy write/audit was not atomic and redacted", count, err)
	}
}

func TestHourlyPolicySharedWalletFenceBlocksConfigurationUpdate(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	actor := createTestUser(t, store, 498022)
	current, err := store.CompareAndSwapHourlyPolicy(ctx, actor.ID, "", "encrypted-original")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	version, err := lockHourlyPolicyVersionTx(ctx, tx)
	if err != nil || version != current.Version {
		t.Fatal("wallet fence did not bind exact stored policy", err)
	}
	deadline, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	_, err = store.CompareAndSwapHourlyPolicy(deadline, actor.ID, current.Version, "encrypted-replacement")
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("configuration write bypassed the shared wallet lock", err)
	}
	unchanged, err := store.ReadHourlyPolicyRecord(ctx)
	if err != nil || unchanged != current {
		t.Fatal("cancelled policy update changed configuration", err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	updated, err := store.CompareAndSwapHourlyPolicy(ctx, actor.ID, current.Version, "encrypted-replacement")
	if err != nil || updated.Version == current.Version {
		t.Fatal("released wallet fence prevented a valid update", err)
	}
}

func TestHourlyPolicyRejectsUnencryptedOrEmptyExistingRows(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	actor := createTestUser(t, store, 498023)
	for _, value := range []struct {
		ciphertext string
		encrypted  bool
	}{{"plaintext", false}, {"", true}} {
		if err := store.PutSetting(ctx, HourlyPolicySettingKey, value.ciphertext, value.encrypted, &actor.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := store.ReadHourlyPolicyRecord(ctx); !errors.Is(err, ErrConflict) {
			t.Fatal("invalid policy row was treated as unconfigured", err)
		}
		if _, err := store.CompareAndSwapHourlyPolicy(ctx, actor.ID, "", "replacement"); !errors.Is(err, ErrConflict) {
			t.Fatal("invalid stored row was overwritten without a version", err)
		}
	}
}
