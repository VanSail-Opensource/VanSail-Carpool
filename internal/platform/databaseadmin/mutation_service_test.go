package databaseadmin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/testdatabase"
	"log/slog"
	"time"

	"testing"
)

func TestProtectedEvidenceAndSecrets(t *testing.T) {
	s, _, backups, _, actor := newTestService(t)
	for _, table := range []string{"settings", "ledger_entries", "purchases", "audit_events", "xnode_payg_accounts", "database_admin_reviews"} {
		_, err := s.ReviewMutation(context.Background(), actor, MutationRequest{Action: "insert", Table: table, Reason: "review protected record"})
		var protected *ProtectionError
		if !errors.As(err, &protected) {
			t.Fatalf("%s protection: %v", table, err)
		}
	}
	if backups.calls != 0 {
		t.Fatal("protected edit created backup")
	}
}

func TestReviewedPostgresCRUD(t *testing.T) {
	s, db, backups, _, actor := newTestService(t)
	ctx := context.Background()
	_, err := db.ExecContext(ctx, `CREATE TABLE editor_records(id TEXT PRIMARY KEY,label TEXT NOT NULL,amount NUMERIC(30,0) NOT NULL,enabled BOOLEAN NOT NULL DEFAULT true)`)
	if err != nil {
		t.Fatal(err)
	}
	apply := func(request MutationRequest) MutationResult {
		t.Helper()
		review, err := s.ReviewMutation(ctx, actor, request)
		if err != nil {
			t.Fatal(err)
		}
		request.ReviewHash, request.Confirmation = review.ReviewHash, review.RequiredConfirmation
		result, err := s.ApplyMutation(ctx, actor, request)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.ApplyMutation(ctx, actor, request); !errors.Is(err, ErrReviewConflict) {
			t.Fatalf("review replay: %v", err)
		}
		return result
	}
	row := apply(MutationRequest{Action: "insert", Table: "editor_records", Reason: "restore operational record", Values: map[string]Value{"id": StringValue("one"), "label": StringValue("before"), "amount": StringValue("900719925474099312345678")}}).Row
	amount, _ := row.Values["amount"].Text()
	if amount != "900719925474099312345678" {
		t.Fatal(amount)
	}
	request := MutationRequest{Action: "update", Table: "editor_records", Key: row.Key, ExpectedHash: row.RecordHash, Reason: "repair operational label", Values: map[string]Value{"label": StringValue("after")}}
	review, err := s.ReviewMutation(ctx, actor, request)
	if err != nil {
		t.Fatal(err)
	}
	request.ReviewHash, request.Confirmation = review.ReviewHash, review.RequiredConfirmation
	if _, err = s.ApplyMutation(ctx, "different-actor", request); !errors.Is(err, ErrReviewConflict) {
		t.Fatalf("actor binding: %v", err)
	}
	if _, err = db.ExecContext(ctx, `UPDATE editor_records SET label='concurrent' WHERE id='one'`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApplyMutation(ctx, actor, request); !errors.Is(err, ErrOptimisticConflict) {
		t.Fatalf("concurrent edit: %v", err)
	}
	page, err := s.Records(ctx, "editor_records", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	row = &page.Items[0]
	request.ExpectedHash = row.RecordHash
	request.Values = map[string]Value{"id": StringValue("replacement")}
	var protected *ProtectionError
	if _, err = s.ReviewMutation(ctx, actor, request); !errors.As(err, &protected) || protected.Reason != "primaryKey" {
		t.Fatalf("key protection: %v", err)
	}
	result := apply(MutationRequest{Action: "delete", Table: "editor_records", Key: row.Key, ExpectedHash: row.RecordHash, Reason: "remove operational record"})
	if !result.Deleted || backups.calls != 2 {
		t.Fatal(result, backups.calls)
	}
	var detail string
	if err = db.QueryRowContext(ctx, `SELECT detail FROM audit_events WHERE action='database_record_insert'`).Scan(&detail); err != nil {
		t.Fatal(err)
	}
	var audit map[string]any
	if json.Unmarshal([]byte(detail), &audit) != nil || audit["values"] != "[redacted]" || audit["reason"] != "[redacted]" {
		t.Fatal(detail)
	}
}

func TestDatabaseBackupFailurePreventsMutation(t *testing.T) {
	s, db, backups, _, actor := newTestService(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `CREATE TABLE editor_records(id TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	request := MutationRequest{Action: "insert", Table: "editor_records", Reason: "restore operational record", Values: map[string]Value{"id": StringValue("one")}}
	review, err := s.ReviewMutation(ctx, actor, request)
	if err != nil {
		t.Fatal(err)
	}
	request.ReviewHash, request.Confirmation = review.ReviewHash, review.RequiredConfirmation
	backups.fail = true
	if _, err = s.ApplyMutation(ctx, actor, request); err == nil {
		t.Fatal("backup failure allowed mutation")
	}
	var count int
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM editor_records`).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	s.now = func() time.Time { return time.Now().Add(11 * time.Minute) }
	backups.fail = false
	if _, err = s.ApplyMutation(ctx, actor, request); !errors.Is(err, ErrReviewConflict) {
		t.Fatalf("expired review: %v", err)
	}
}

func newTestService(t *testing.T) (*Service, *sql.DB, *fakeBackupper, *secret.Vault, string) {
	t.Helper()
	ctx := context.Background()
	db, err := testdatabase.Open(ctx, t)
	if err != nil {
		t.Fatalf("database.Open(): %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store := database.NewStore(db)
	user, _, err := store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 424242, FirstName: "Admin"}, true)
	if err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	vault, err := secret.NewVault([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("secret.NewVault(): %v", err)
	}
	backup := &fakeBackupper{}
	return NewService(db, backup, vault, slog.Default()), db, backup, vault, user.ID
}

type fakeBackupper struct {
	calls int
	fail  bool
}

func (b *fakeBackupper) Run(_ context.Context) (model.BackupRun, error) {
	b.calls++
	if b.fail {
		return model.BackupRun{}, errors.New("backup unavailable")
	}
	now := time.Now()
	return model.BackupRun{SizeBytes: 4096, CompletedAt: &now, ID: "rescue-" + string(rune('0'+b.calls)), Status: "complete"}, nil
}
