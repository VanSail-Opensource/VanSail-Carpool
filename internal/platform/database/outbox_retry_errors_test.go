package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestOutboxRetryPreservesStorageErrors(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	actor := createTestUser(t, s, 95006)
	now := time.Now().UTC()
	operation, _, err := s.CreateAdminMutation(ctx, adminMutationTestInput(actor.ID, "storage-error"), now)
	if err != nil {
		t.Fatal(err)
	}
	var jobID string
	if err := s.db.QueryRowContext(ctx, "SELECT id FROM outbox_jobs WHERE kind='provider_operation'").Scan(&jobID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, "UPDATE outbox_jobs SET status='failed' WHERE id=$1", jobID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, "ALTER TABLE provider_operation_items RENAME TO unavailable_operation_items"); err != nil {
		t.Fatal(err)
	}
	err = s.CompleteOutboxRetryOperation(ctx, operation.Receipt.ID, "target", jobID, now)
	var postgresError *pgconn.PgError
	if errors.Is(err, ErrConflict) || !errors.As(err, &postgresError) || postgresError.Code != "42P01" {
		t.Fatalf("storage error collapsed into a business conflict: %v", err)
	}
	if _, err := s.db.ExecContext(ctx, "ALTER TABLE unavailable_operation_items RENAME TO provider_operation_items"); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := s.db.QueryRowContext(ctx, "SELECT status FROM outbox_jobs WHERE id=$1", jobID).Scan(&status); err != nil || status != "failed" {
		t.Fatal("failed read changed the target job", status, err)
	}
}
