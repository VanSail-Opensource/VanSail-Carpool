package database

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"testing"
	"time"
)

func TestDefinitelyRejectedCommandRetainsFirstAttemptOnRetry(t *testing.T) {
	s := newTestStore(t)
	owner := createTestUser(t, s, 50123)
	ctx := context.Background()
	now := time.Now().UTC()
	op, _, err := s.CreateProviderOperation(ctx, providerops.CreateInput{ActorUserID: owner.ID, Kind: xnode.OperationKind, IdempotencyKey: "busy-command", RequestFingerprint: "busy-command-fingerprint", SealedTarget: "sealed", Items: []providerops.ItemInput{{Key: "node", TargetType: "xnode", TargetID: "node"}}}, now)
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		op, err = s.BeginProviderOperationAttempt(ctx, op.Receipt.ID, now)
		if err != nil || op.Attempts != 1 {
			t.Fatalf("rejection consumed attempt: %d %v", op.Attempts, err)
		}
		if err = s.DeferXNodeCommand(ctx, op.Receipt.ID, op.Attempts, now); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.DeferXNodeCommand(ctx, op.Receipt.ID, 1, now); err == nil {
		t.Fatal("stale callback released another attempt")
	}
}
