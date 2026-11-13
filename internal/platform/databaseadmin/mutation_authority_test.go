package databaseadmin

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestDatabaseMutationRequiresCurrentAdministrator(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"demoted", "purging", "blocked"} {
		t.Run(state, func(t *testing.T) {
			service, db, backups, _, actor := newTestService(t)
			request := reviewedAuthorityMutation(t, service, db, actor)
			setMutationActorState(t, db, actor, state)

			if _, err := service.ReviewMutation(context.Background(), actor, request); !errors.Is(err, ErrReviewConflict) {
				t.Fatalf("ineligible actor admitted a new review: %v", err)
			}
			if _, err := service.ApplyMutation(context.Background(), actor, request); !errors.Is(err, ErrReviewConflict) {
				t.Fatalf("retained review bypassed current authority: %v", err)
			}
			if backups.calls != 0 {
				t.Fatal("ineligible actor started a rescue backup")
			}
			assertAuthorityMutationUnchanged(t, db, actor, request.ReviewHash)
			setMutationActorState(t, db, actor, "active")
			if result, err := service.ApplyMutation(context.Background(), actor, request); err != nil || result.Row == nil {
				t.Fatalf("restored actor could not use unconsumed review: %+v, %v", result, err)
			}
		})
	}
}

func TestDatabaseMutationRechecksAuthorityAfterRescueBackup(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"demoted", "purging", "blocked"} {
		t.Run(state, func(t *testing.T) {
			service, db, backups, _, actor := newTestService(t)
			request := reviewedAuthorityMutation(t, service, db, actor)
			service.backups = authorityBackupFunc(func(ctx context.Context) (model.BackupRun, error) {
				setMutationActorState(t, db, actor, state)
				return backups.Run(ctx)
			})
			if _, err := service.ApplyMutation(context.Background(), actor, request); !errors.Is(err, ErrReviewConflict) {
				t.Fatalf("authority changed during backup but edit committed: %v", err)
			}
			if backups.calls != 1 {
				t.Fatal("fixture did not reach the rescue backup")
			}
			assertAuthorityMutationUnchanged(t, db, actor, request.ReviewHash)
			setMutationActorState(t, db, actor, "active")
			service.backups = backups
			if _, err := service.ApplyMutation(context.Background(), actor, request); err != nil {
				t.Fatalf("rejected attempt consumed its exact review: %v", err)
			}
		})
	}
}

func TestDatabaseMutationRejectsMissingActorBeforeReview(t *testing.T) {
	t.Parallel()
	service, db, backups, _, actor := newTestService(t)
	request := reviewedAuthorityMutation(t, service, db, actor)
	for _, missing := range []string{"", " ", "missing-admin"} {
		if _, err := service.ReviewMutation(context.Background(), missing, request); !errors.Is(err, ErrReviewConflict) {
			t.Fatalf("missing actor admitted review: %v", err)
		}
		if _, err := service.ApplyMutation(context.Background(), missing, request); !errors.Is(err, ErrReviewConflict) {
			t.Fatalf("missing actor used retained review: %v", err)
		}
	}
	if backups.calls != 0 {
		t.Fatal("missing actor started backup")
	}
	assertAuthorityMutationUnchanged(t, db, actor, request.ReviewHash)
}

type authorityBackupFunc func(context.Context) (model.BackupRun, error)

func (f authorityBackupFunc) Run(ctx context.Context) (model.BackupRun, error) { return f(ctx) }

func reviewedAuthorityMutation(t *testing.T, service *Service, db *sql.DB, actor string) MutationRequest {
	t.Helper()
	if _, err := db.Exec(`CREATE TABLE authority_editor_records(id TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	request := MutationRequest{Action: "insert", Table: "authority_editor_records",
		Reason: "restore operational record", Values: map[string]Value{"id": StringValue("one")}}
	review, err := service.ReviewMutation(context.Background(), actor, request)
	if err != nil {
		t.Fatal(err)
	}
	request.ReviewHash, request.Confirmation = review.ReviewHash, review.RequiredConfirmation
	return request
}

func setMutationActorState(t *testing.T, db *sql.DB, actor, state string) {
	t.Helper()
	role, status, blocked := "admin", "active", 0
	switch state {
	case "demoted":
		role = "user"
	case "purging":
		status = "purging"
	case "blocked":
		blocked = 1
	}
	if _, err := db.Exec(`UPDATE users SET role=$1,account_status=$2,bot_blocked=$3 WHERE id=$4`, role, status, blocked, actor); err != nil {
		t.Fatal(err)
	}
}

func assertAuthorityMutationUnchanged(t *testing.T, db *sql.DB, actor, review string) {
	t.Helper()
	var rows, audits, unconsumed, reviews int
	if err := db.QueryRow(`SELECT COUNT(*) FROM authority_editor_records`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_events WHERE actor_user_id=$1 AND action='database_record_insert' AND target_id='authority_editor_records'`, actor).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM database_admin_reviews WHERE id=$1 AND consumed_at IS NULL`, review).Scan(&unconsumed); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM database_admin_reviews WHERE actor_user_id=$1`, actor).Scan(&reviews); err != nil {
		t.Fatal(err)
	}
	if rows != 0 || audits != 0 || unconsumed != 1 || reviews != 1 {
		t.Fatalf("rejected actor changed evidence: rows=%d audits=%d unconsumed=%d reviews=%d", rows, audits, unconsumed, reviews)
	}
}
