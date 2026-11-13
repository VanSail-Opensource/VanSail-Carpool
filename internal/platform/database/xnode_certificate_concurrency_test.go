package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestCertificateOperationConcurrentAdministratorsQueueOneIntent(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	actors := []string{createTestAdmin(t, store, 871005).ID, createTestAdmin(t, store, 871006).ID}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	start, results := make(chan struct{}), make(chan error, len(actors))
	for _, actor := range actors {
		go func() {
			<-start
			input := certificateOperationInput(actor, "concurrent", "node", "primary")
			_, _, err := store.CreateXNodeCertificateOperation(ctx, input, "primary", "renew", 7, time.Now().UTC())
			results <- err
		}()
	}
	close(start)
	success, conflicts := 0, 0
	for range actors {
		err := <-results
		if err == nil {
			success++
		} else if errors.Is(err, xnode.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatalf("success=%d conflicts=%d, want one of each", success, conflicts)
	}
	assertCertificateQueueCount(t, store, 1)
}

func TestCertificateOperationProfileLockHonorsCancellation(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	actor := createTestAdmin(t, store, 871007)
	tx, err := store.DB().BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(context.Background(), `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "xnode.certificate:node:primary"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	input := certificateOperationInput(actor.ID, "cancelled", "node", "primary")
	if _, _, err := store.CreateXNodeCertificateOperation(ctx, input, "primary", "issue", 7, time.Now().UTC()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("profile lock ignored cancellation: %v", err)
	}
	assertCertificateQueueCount(t, store, 0)
}
