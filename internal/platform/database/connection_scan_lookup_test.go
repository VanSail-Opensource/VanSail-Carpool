package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connections"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestConnectionScanLookupClassifiesOnlyMissingEvidence(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	owner := createTestUser(t, store, 900_601)
	ctx := context.Background()
	now := time.Now().UTC()
	scan, _, err := store.CreateConnectionScan(ctx, providerops.ConnectionScanInput{ActorUserID: scanAdministrator(t, store).ID,
		UserID: owner.ID, IdempotencyKey: "lookup-contract", RequestFingerprint: "0123456789abcdef", ExpiresAt: now.Add(time.Minute),
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []struct{ id, owner string }{{"missing", owner.ID}, {scan.ID, "different-owner"}} {
		if _, err := store.ConnectionScanForUser(ctx, target.id, target.owner); !errors.Is(err, ErrNotFound) || !errors.Is(err, connections.ErrScanNotFound) {
			t.Fatal("missing/foreign scan classification", target, err)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := store.ConnectionScanForUser(cancelled, scan.ID, owner.ID); !errors.Is(err, context.Canceled) || errors.Is(err, ErrNotFound) || errors.Is(err, connections.ErrScanNotFound) {
		t.Fatal("transient query error became not found", err)
	}
	if current, err := store.ConnectionScanForUser(ctx, scan.ID, owner.ID); err != nil || current.ID != scan.ID {
		t.Fatal("healthy retry lost retained scan", current, err)
	}
}
