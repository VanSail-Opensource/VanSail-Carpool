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

	"testing"
)

func TestPostgresAdministrationCannotMutate(t *testing.T) {
	s, db, backups, vault, actor := newTestService(t)
	cipher, err := vault.Encrypt("provider.secret", "protected")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(context.Background(), `INSERT INTO settings(key,value,encrypted,updated_at) VALUES('provider.secret',$1,1,'2026-01-01T00:00:00Z')`, cipher); err != nil {
		t.Fatal(err)
	}
	page, err := s.Records(context.Background(), "settings", "", 200)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range page.Items {
		if row.Key["key"] == "provider.secret" {
			raw, _ := json.Marshal(row.Values["value"])
			if string(raw) != `"********"` {
				t.Fatal("secret not masked")
			}
		}
	}
	request := MutationRequest{Action: "delete", Table: "users", Key: map[string]string{"id": actor}, Reason: "attempt generic write"}
	if _, err = s.ReviewMutation(context.Background(), actor, request); !errors.Is(err, ErrInvalidValue) {
		t.Fatal(err)
	}
	if _, err = s.ApplyMutation(context.Background(), actor, request); !errors.Is(err, ErrInvalidValue) {
		t.Fatal(err)
	}
	if backups.calls != 0 {
		t.Fatal("disabled mutation created a backup")
	}
	var count int
	if err = db.QueryRowContext(context.Background(), `SELECT count(*) FROM users WHERE id=$1`, actor).Scan(&count); err != nil || count != 1 {
		t.Fatal("write changed data", err)
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

type fakeBackupper struct{ calls int }

func (b *fakeBackupper) Run(_ context.Context) (model.BackupRun, error) {
	b.calls++
	return model.BackupRun{ID: "rescue-" + string(rune('0'+b.calls)), Status: "complete"}, nil
}
