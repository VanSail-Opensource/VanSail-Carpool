package backup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/testdatabase"
)

func TestBackupDeletionRequiresCurrentEligibleAdministrator(t *testing.T) {
	for _, condition := range []string{"active", "demoted", "purging", "blocked", "missing", "blank"} {
		t.Run(condition, func(t *testing.T) {
			ctx := context.Background()
			db, err := testdatabase.Open(ctx, t)
			if err != nil {
				t.Fatal(err)
			}
			store := database.NewStore(db)
			actor, _, err := store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 99118, FirstName: "Operator"}, true)
			if err != nil {
				t.Fatal(err)
			}
			directory := t.TempDir()
			path := filepath.Join(directory, "tx-carpool-reviewed.dump")
			if err := os.WriteFile(path, []byte("retained archive"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := db.ExecContext(ctx, `INSERT INTO backup_runs(id,path,status,created_at) VALUES('reviewed',$1,'complete',$2)`, path, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
				t.Fatal(err)
			}
			service := NewService(db, store, directory, time.Hour)
			actorID := actor.ID
			switch condition {
			case "demoted":
				_, err = db.ExecContext(ctx, `UPDATE users SET role='user' WHERE id=$1`, actorID)
			case "purging":
				_, err = db.ExecContext(ctx, `UPDATE users SET account_status='purging' WHERE id=$1`, actorID)
			case "blocked":
				_, err = db.ExecContext(ctx, `UPDATE users SET bot_blocked=1 WHERE id=$1`, actorID)
			case "missing":
				actorID = "missing-actor"
			case "blank":
				actorID = " "
			}
			if err != nil {
				t.Fatal(err)
			}
			err = service.Delete(ctx, "reviewed", actorID)
			var retained, audits int
			if queryErr := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM backup_runs WHERE id='reviewed'`).Scan(&retained); queryErr != nil {
				t.Fatal(queryErr)
			}
			if queryErr := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE action='backup.delete' AND target_id='reviewed'`).Scan(&audits); queryErr != nil {
				t.Fatal(queryErr)
			}
			data, fileErr := os.ReadFile(path)
			if condition == "active" {
				if err != nil || retained != 0 || audits != 1 || !errors.Is(fileErr, os.ErrNotExist) {
					t.Fatal("eligible deletion lost atomic metadata/audit", err, retained, audits, fileErr)
				}
			} else if !errors.Is(err, ErrBackupForbidden) || retained != 1 || audits != 0 || fileErr != nil || string(data) != "retained archive" {
				t.Fatal("ineligible actor changed backup evidence", err, retained, audits, fileErr)
			}
		})
	}
}
