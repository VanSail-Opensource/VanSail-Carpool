package databaseadmin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// A retained review binds the request, but authority must still be current.
// Transaction callers hold this fence through review admission or edit/audit
// commit; preflight callers avoid starting a rescue backup for a stale actor.
func requireMutationActor(ctx context.Context, reader queryRower, actor string) error {
	if strings.TrimSpace(actor) == "" {
		return ErrReviewConflict
	}
	query := `SELECT role,account_status,bot_blocked=1 FROM users WHERE id=$1`
	if _, transaction := reader.(*sql.Tx); transaction {
		query += ` FOR SHARE`
	}
	var role, status string
	var blocked bool
	err := reader.QueryRowContext(ctx, query, actor).Scan(&role, &status, &blocked)
	if errors.Is(err, sql.ErrNoRows) || err == nil && (role != "admin" || status != "active" || blocked) {
		return ErrReviewConflict
	}
	if err != nil {
		return fmt.Errorf("verify database mutation actor: %w", err)
	}
	return nil
}
