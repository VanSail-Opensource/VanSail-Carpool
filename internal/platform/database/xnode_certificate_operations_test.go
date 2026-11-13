package database

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func certificateOperationInput(actor, key, node, profile string) providerops.CreateInput {
	return providerops.CreateInput{ActorUserID: actor, Kind: "xnode_command", IdempotencyKey: key,
		RequestFingerprint: "certificate-request-" + key, SealedTarget: "synthetic-private-command",
		Items: []providerops.ItemInput{{Key: "certificate:" + profile, TargetType: "xnode", TargetID: node}}}
}

func assertCertificateQueueCount(t *testing.T, store *Store, operations int) {
	t.Helper()
	for _, query := range []string{
		`SELECT COUNT(*) FROM provider_operations WHERE kind='xnode_command'`,
		`SELECT COUNT(*) FROM provider_operation_items WHERE item_key LIKE 'certificate:%'`,
		`SELECT COUNT(*) FROM outbox_jobs WHERE kind='provider_operation'`,
	} {
		var count int
		if err := store.DB().QueryRowContext(context.Background(), query).Scan(&count); err != nil || count != operations {
			t.Fatalf("atomic queue count=%d want=%d query=%s error=%v", count, operations, query, err)
		}
	}
}

func TestCertificateOperationInitialIntentAndLatestScope(t *testing.T) {
	t.Parallel()
	store, ctx := newTestStore(t), context.Background()
	actor := createTestAdmin(t, store, 871001)
	now := time.Now().UTC().Truncate(time.Microsecond)
	input := certificateOperationInput(actor.ID, "initial", "node-one", "primary")
	op, replayed, err := store.CreateXNodeCertificateOperation(ctx, input, "primary", "issue", 9007199254740993, now)
	if err != nil || replayed || op.Receipt.Status != "queued" {
		t.Fatalf("create=%+v replay=%t error=%v", op, replayed, err)
	}
	var intent struct {
		Command  string `json:"command"`
		NodeID   string `json:"nodeId"`
		Profile  string `json:"profile_id"`
		Action   string `json:"action"`
		Revision uint64 `json:"expected_revision"`
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal([]byte(op.ResultJSON), &intent) != nil || json.Unmarshal([]byte(op.ResultJSON), &fields) != nil || len(fields) != 5 || intent.Command != "certificates.run" || intent.NodeID != "node-one" || intent.Profile != "primary" || intent.Action != "issue" || intent.Revision != 9007199254740993 {
		t.Fatalf("unsafe/inexact certificate intent: %s", op.ResultJSON)
	}
	latest, err := store.LatestXNodeCertificateOperation(ctx, "node-one", "primary")
	if err != nil || latest.Receipt.ID != op.Receipt.ID || latest.ResultJSON != op.ResultJSON {
		t.Fatalf("latest=%+v error=%v", latest, err)
	}
	for _, scope := range [][2]string{{"node-two", "primary"}, {"node-one", "secondary"}} {
		if _, err := store.LatestXNodeCertificateOperation(ctx, scope[0], scope[1]); !errors.Is(err, xnode.ErrNotFound) {
			t.Fatalf("different certificate scope leaked: %v", err)
		}
	}
	assertCertificateQueueCount(t, store, 1)
}

func TestCertificateOperationReplayPrecedesUncertainStatusGuard(t *testing.T) {
	for _, status := range []string{"queued", "processing", "pending_review", "partial"} {
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			store, ctx := newTestStore(t), context.Background()
			actor := createTestAdmin(t, store, 871002)
			other := createTestAdmin(t, store, 871003)
			now := time.Now().UTC()
			input := certificateOperationInput(actor.ID, "first", "node", "primary")
			op, _, err := store.CreateXNodeCertificateOperation(ctx, input, "primary", "renew", 7, now)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = store.DB().ExecContext(ctx, `UPDATE provider_operations SET status=$1 WHERE id=$2`, status, op.Receipt.ID); err != nil {
				t.Fatal(err)
			}
			replay, replayed, err := store.CreateXNodeCertificateOperation(ctx, input, "primary", "renew", 7, now.Add(time.Second))
			if err != nil || !replayed || replay.Receipt.ID != op.Receipt.ID || replay.ResultJSON != op.ResultJSON || replay.Receipt.Status != status {
				t.Fatalf("replay=%+v replayed=%t error=%v", replay, replayed, err)
			}
			conflict := input
			conflict.RequestFingerprint = "different-certificate-request"
			if _, _, err := store.CreateXNodeCertificateOperation(ctx, conflict, "primary", "renew", 7, now); !errors.Is(err, ErrConflict) || !errors.Is(err, xnode.ErrConflict) {
				t.Fatalf("fingerprint mismatch did not conflict: %v", err)
			}
			fresh := certificateOperationInput(other.ID, "second", "node", "primary")
			if _, _, err := store.CreateXNodeCertificateOperation(ctx, fresh, "primary", "issue", 8, now.Add(time.Minute)); !errors.Is(err, xnode.ErrConflict) {
				t.Fatalf("uncertain profile accepted a fresh cross-admin operation: %v", err)
			}
			assertCertificateQueueCount(t, store, 1)
		})
	}
}

func TestCertificateOperationTerminalReceiptAllowsExplicitNewRequest(t *testing.T) {
	for _, status := range []string{"failed", "succeeded"} {
		t.Run(status, func(t *testing.T) {
			store, ctx := newTestStore(t), context.Background()
			actor := createTestAdmin(t, store, 871004)
			now := time.Now().UTC().Truncate(time.Second)
			first := certificateOperationInput(actor.ID, "first", "node", "primary")
			op, _, err := store.CreateXNodeCertificateOperation(ctx, first, "primary", "issue", 7, now)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = store.DB().ExecContext(ctx, `UPDATE provider_operations SET status=$1 WHERE id=$2`, status, op.Receipt.ID); err != nil {
				t.Fatal(err)
			}
			second := certificateOperationInput(actor.ID, "second", "node", "primary")
			next, replayed, err := store.CreateXNodeCertificateOperation(ctx, second, "primary", "renew", 8, now.Add(time.Nanosecond))
			if err != nil || replayed || next.Receipt.ID == op.Receipt.ID {
				t.Fatalf("new explicit request=%+v replay=%t error=%v", next, replayed, err)
			}
			latest, err := store.LatestXNodeCertificateOperation(ctx, "node", "primary")
			if err != nil || latest.Receipt.ID != next.Receipt.ID {
				t.Fatalf("latest did not recover the newest receipt: %+v %v", latest, err)
			}
			replay, replayed, err := store.CreateXNodeCertificateOperation(ctx, first, "primary", "issue", 7, now.Add(2*time.Second))
			if err != nil || !replayed || replay.Receipt.ID != op.Receipt.ID || replay.Receipt.Status != status {
				t.Fatalf("prior completed receipt did not replay while successor active: %+v %t %v", replay, replayed, err)
			}
			assertCertificateQueueCount(t, store, 2)
		})
	}
}

func TestCertificateOperationFailedReceiptRetainsSafeIntent(t *testing.T) {
	t.Parallel()
	store, ctx := newTestStore(t), context.Background()
	actor := createTestAdmin(t, store, 871008)
	now := time.Now().UTC()
	input := certificateOperationInput(actor.ID, "preflight-failed", "node", "primary")
	op, _, err := store.CreateXNodeCertificateOperation(ctx, input, "primary", "renew", 7, now)
	if err != nil {
		t.Fatal(err)
	}
	// The worker echoes immutable intent even when preflight fails before attempt.
	if _, err = store.CompleteProviderOperation(ctx, op.Receipt.ID, providerops.Completion{Status: providerops.StatusFailed, ErrorCode: "XNODE_UNAVAILABLE", ResultJSON: op.ResultJSON}, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	latest, err := store.LatestXNodeCertificateOperation(ctx, "node", "primary")
	if err != nil || latest.Receipt.Status != "failed" || latest.ResultJSON != op.ResultJSON {
		t.Fatalf("failed intent was lost: %+v %v", latest, err)
	}
	assertCertificateQueueCount(t, store, 1)
}

func TestCertificateOperationRejectsUnavailableOwnerAtomically(t *testing.T) {
	t.Parallel()
	store, ctx := newTestStore(t), context.Background()
	actor := createTestAdmin(t, store, 871009)
	owner := createTestUser(t, store, 871010)
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET account_status='purged' WHERE id=$1`, owner.ID); err != nil {
		t.Fatal(err)
	}
	input := certificateOperationInput(actor.ID, "inactive-owner", "node", "primary")
	input.OwnerUserID = owner.ID
	if _, _, err := store.CreateXNodeCertificateOperation(ctx, input, "primary", "issue", 7, time.Now().UTC()); !errors.Is(err, ErrAccountUnavailable) {
		t.Fatalf("unavailable owner created a certificate operation: %v", err)
	}
	assertCertificateQueueCount(t, store, 0)
}

func TestNativeCommandAdmissionRechecksAdministratorAndPreservesReplay(t *testing.T) {
	for _, certificate := range []bool{false, true} {
		t.Run(map[bool]string{false: "command", true: "certificate"}[certificate], func(t *testing.T) {
			store, ctx := newTestStore(t), context.Background()
			actor := createTestAdmin(t, store, 871011)
			now := time.Now().UTC()
			create := func(key string) (providerops.Operation, bool, error) {
				input := certificateOperationInput(actor.ID, key, "node", "primary")
				if certificate {
					return store.CreateXNodeCertificateOperation(ctx, input, "primary", "issue", 7, now)
				}
				return store.CreateProviderOperation(ctx, input, now)
			}
			accepted, replayed, err := create("accepted")
			if err != nil || replayed {
				t.Fatal(accepted, replayed, err)
			}
			if _, err = store.DB().ExecContext(ctx, `UPDATE users SET role='user' WHERE id=$1`, actor.ID); err != nil {
				t.Fatal(err)
			}
			if _, _, err = create("after-demotion"); !errors.Is(err, ErrConflict) {
				t.Fatal("stale administrator queued a native command", err)
			}
			replayedOperation, replayed, err := create("accepted")
			if err != nil || !replayed || replayedOperation.Receipt.ID != accepted.Receipt.ID {
				t.Fatal("later demotion changed committed intent", replayedOperation, replayed, err)
			}
			assertCertificateQueueCount(t, store, 1)
		})
	}
}
