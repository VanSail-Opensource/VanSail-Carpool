package databaseadmin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReviewedUpdateCreatesRescueAuditAndCannotReplay(t *testing.T) {
	t.Parallel()
	service, db, backup, _, actor := newTestService(t)
	ctx := context.Background()
	page, err := service.Records(ctx, "users", "", 10)
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("Records(users) = %+v, %v", page, err)
	}
	row := page.Items[0]
	request := MutationRequest{Action: "update", Table: "users", Key: row.Key, ExpectedHash: row.RecordHash,
		Values: map[string]Value{"telegram_first_name": StringValue("Reviewed")}, Reason: "Correct the imported display name"}
	review, err := service.ReviewMutation(ctx, actor, request)
	if err != nil {
		t.Fatalf("ReviewMutation(): %v", err)
	}
	if review.ReviewHash == "" || review.RequiredConfirmation != "EDIT users" || len(review.ChangedColumns) != 1 {
		t.Fatalf("review = %+v", review)
	}
	request.ReviewHash, request.Confirmation = review.ReviewHash, review.RequiredConfirmation
	result, err := service.ApplyMutation(ctx, actor, request)
	if err != nil {
		t.Fatalf("ApplyMutation(): %v", err)
	}
	if backup.calls != 1 || result.RescueBackupID == "" || result.Row == nil {
		t.Fatalf("backup calls=%d, result=%+v", backup.calls, result)
	}
	var firstName string
	if err := db.QueryRowContext(ctx, `SELECT telegram_first_name FROM users WHERE id=?`, actor).Scan(&firstName); err != nil || firstName != "Reviewed" {
		t.Fatalf("updated first name=%q, error=%v", firstName, err)
	}
	var detail string
	if err := db.QueryRowContext(ctx, `SELECT detail FROM audit_events WHERE action='database_record_update' ORDER BY created_at DESC LIMIT 1`).Scan(&detail); err != nil {
		t.Fatalf("read audit: %v", err)
	}
	if strings.Contains(detail, "Reviewed") || !strings.Contains(detail, `"values":"[redacted]"`) {
		t.Fatalf("audit detail was not redacted: %s", detail)
	}
	if _, err := service.ApplyMutation(ctx, actor, request); !errors.Is(err, ErrReviewConflict) {
		t.Fatalf("review replay error = %v", err)
	}
}

func TestOptimisticConflictRollsBackReviewAndEdit(t *testing.T) {
	t.Parallel()
	service, db, _, _, actor := newTestService(t)
	ctx := context.Background()
	page, err := service.Records(ctx, "users", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	row := page.Items[0]
	request := MutationRequest{Action: "update", Table: "users", Key: row.Key, ExpectedHash: row.RecordHash,
		Values: map[string]Value{"telegram_last_name": StringValue("planned")}, Reason: "Repair imported account metadata"}
	review, err := service.ReviewMutation(ctx, actor, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE users SET telegram_last_name='concurrent' WHERE id=?`, actor); err != nil {
		t.Fatal(err)
	}
	request.ReviewHash, request.Confirmation = review.ReviewHash, review.RequiredConfirmation
	if _, err := service.ApplyMutation(ctx, actor, request); !errors.Is(err, ErrOptimisticConflict) {
		t.Fatalf("optimistic error = %v", err)
	}
	var lastName string
	if err := db.QueryRowContext(ctx, `SELECT telegram_last_name FROM users WHERE id=?`, actor).Scan(&lastName); err != nil || lastName != "concurrent" {
		t.Fatalf("last name=%q, error=%v", lastName, err)
	}
	var consumed sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT consumed_at FROM database_admin_reviews WHERE id=?`, review.ReviewHash).Scan(&consumed); err != nil || consumed.Valid {
		t.Fatalf("review consumed=%v, error=%v", consumed.Valid, err)
	}
}

func TestEncryptedSettingIsMaskedAndReplacedThroughVault(t *testing.T) {
	t.Parallel()
	service, db, _, vault, actor := newTestService(t)
	ctx := context.Background()
	initial, err := vault.Encrypt("provider.secret", "old-secret")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.ExecContext(ctx, `INSERT INTO settings(key,value,encrypted,updated_at,updated_by) VALUES(?,?,?,?,?)`, "provider.secret", initial, 1, now, actor); err != nil {
		t.Fatal(err)
	}
	page, err := service.Records(ctx, "settings", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	var row Record
	for _, candidate := range page.Items {
		if candidate.Key["key"] == "provider.secret" {
			row = candidate
		}
	}
	encoded, err := json.Marshal(row.Values["value"])
	if err != nil || strings.Contains(string(encoded), initial) || string(encoded) != `"********"` {
		t.Fatalf("masked value=%s, error=%v", encoded, err)
	}
	request := MutationRequest{Action: "update", Table: "settings", Key: row.Key, ExpectedHash: row.RecordHash,
		Values: map[string]Value{"value": StringValue("new-secret")}, Reason: "Rotate the provider credential"}
	review, err := service.ReviewMutation(ctx, actor, request)
	if err != nil {
		t.Fatal(err)
	}
	request.ReviewHash, request.Confirmation = review.ReviewHash, review.RequiredConfirmation
	if _, err := service.ApplyMutation(ctx, actor, request); err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key='provider.secret'`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == "new-secret" || strings.Contains(stored, "new-secret") {
		t.Fatalf("setting stored plaintext: %q", stored)
	}
	plaintext, err := vault.Decrypt("provider.secret", stored)
	if err != nil || plaintext != "new-secret" {
		t.Fatalf("decrypted setting=%q, error=%v", plaintext, err)
	}
}

func newTestService(t *testing.T) (*Service, *sql.DB, *fakeBackupper, *secret.Vault, string) {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "database-admin.db")
	db, err := database.Open(ctx, path)
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

type fakeBackupper struct{ calls int }

func (b *fakeBackupper) Run(_ context.Context) (model.BackupRun, error) {
	b.calls++
	return model.BackupRun{ID: "rescue-" + string(rune('0'+b.calls)), Status: "complete"}, nil
}
