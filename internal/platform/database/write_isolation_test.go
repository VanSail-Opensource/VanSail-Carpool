package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestBlockedMemberWriteDoesNotBlockIndependentLeaseOrLogout(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	store, service, owner, _, now := ipLookupFixture(t, 0)
	op := submitIP(t, service, owner, "isolated-writes", false)
	if claimed, err := store.ClaimIPLookupLease(ctx, op.ID, "worker", now); err != nil || !claimed {
		t.Fatal(claimed, err)
	}
	hash := []byte("independent-browser-device")
	if err := store.CreateBrowserSession(ctx, hash, owner, now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	blocker, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = blocker.Rollback() }()
	if _, err := blocker.ExecContext(ctx, `SELECT 1 FROM users WHERE id=$1 FOR UPDATE`, owner); err != nil {
		t.Fatal(err)
	}
	blocked := make(chan error, 1)
	go func() {
		_, err := store.UpdateMembership(ctx, owner, true, true)
		blocked <- err
	}()
	waitStoreWriteBlocked(t, ctx, store, "UPDATE users SET group_joined=")
	independent := make(chan error, 2)
	go func() { independent <- store.HeartbeatIPLookup(ctx, op.ID, "worker", now.Add(time.Second)) }()
	go func() { independent <- store.DeleteBrowserSession(ctx, hash) }()
	for range 2 {
		select {
		case err := <-independent:
			if err != nil {
				t.Fatal("independent write failed", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("unrelated row lock blocked a lease heartbeat or device revocation")
		}
	}
	if err := blocker.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := <-blocked; err != nil {
		t.Fatal(err)
	}
	var exists bool
	if err := store.DB().QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sessions WHERE token_hash=$1)`, hash).Scan(&exists); err != nil || exists {
		t.Fatal("browser logout did not revoke its session", exists, err)
	}
}

func TestConcurrentItemAdmissionAndCompletionHaveOneWinner(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	owner := createTestUser(t, store, 497726)
	now := time.Now().UTC()
	op, _, err := store.CreateProviderOperation(ctx, providerops.CreateInput{
		ActorUserID: owner.ID, OwnerUserID: owner.ID, Kind: "test_operation",
		IdempotencyKey: "item-contention", RequestFingerprint: "0123456789abcdef",
		Items: []providerops.ItemInput{{Key: "target", TargetType: "purchase", TargetID: "purchase"}},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, completion := range []bool{false, true} {
		start := make(chan struct{})
		results := make(chan error, 8)
		for range 8 {
			go func() {
				<-start
				var item providerops.Item
				var err error
				if completion {
					item, err = store.CompleteProviderOperationItem(ctx, op.Receipt.ID, "target", providerops.Completion{
						Status: providerops.StatusSucceeded, ResultJSON: `{"applied":true}`,
					}, now.Add(time.Second))
				} else {
					item, err = store.BeginProviderOperationItemAttempt(ctx, op.Receipt.ID, "target", now)
				}
				if err == nil && (item.AttemptStartedAt == nil || completion && item.CompletedAt == nil) {
					err = errors.New("winning statement did not return its durable receipt")
				}
				results <- err
			}()
		}
		close(start)
		winners := 0
		for range 8 {
			err := <-results
			if err == nil {
				winners++
			} else if !errors.Is(err, ErrConflict) {
				t.Fatal(err)
			}
		}
		if winners != 1 {
			t.Fatalf("completion=%t had %d winners", completion, winners)
		}
	}
	items, err := store.ProviderOperationItems(ctx, op.Receipt.ID)
	if err != nil || len(items) != 1 || items[0].Status != providerops.StatusSucceeded || items[0].ResultJSON != `{"applied":true}` {
		t.Fatal("winning receipt was not retained", items, err)
	}
}

func waitStoreWriteBlocked(t *testing.T, ctx context.Context, store *Store, query string) {
	t.Helper()
	for {
		var blocked bool
		if err := store.DB().QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database()
			AND pid<>pg_backend_pid() AND wait_event_type='Lock' AND strpos(query,$1)>0)`, query).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("write did not reach its database lock", ctx.Err())
		case <-time.After(5 * time.Millisecond):
		}
	}
}
