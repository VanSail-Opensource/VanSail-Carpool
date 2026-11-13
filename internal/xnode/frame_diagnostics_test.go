package xnode

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestFrameDiagnosticsExposeOnlyAllowlistedCauses(t *testing.T) {
	for _, item := range []struct {
		err  error
		want string
	}{
		{context.DeadlineExceeded, "deadline"}, {sql.ErrNoRows, "missing_record"},
		{ErrInvalid, "invalid_input"}, {ErrConflict, "state_conflict"},
		{&pgconn.PgError{Code: "23505", Message: "secret constraint detail"}, "storage_constraint"},
		{&pgconn.PgError{Code: "40001", Message: "secret transaction detail"}, "storage_busy"},
		{&pgconn.PgError{Code: "40P01", Detail: "secret deadlock query"}, "storage_busy"},
		{&pgconn.PgError{Code: "25006", Message: "secret read-only query"}, "storage_readonly"},
		{&pgconn.PgError{Code: "58030", Detail: "secret path"}, "storage_io"},
		{&pgconn.PgError{Code: "P0001", Message: "unclassified private detail"}, "internal_failure"},
		{errors.New("url?token=private-value"), "internal_failure"},
	} {
		err := firewallEventFailure("XNODE_FIREWALL_EVENT_STORAGE_FAILED", fmt.Errorf("sensitive context: %w", item.err))
		if got := frameErrorCause(err); got != item.want {
			t.Fatalf("safe cause = %q, want %q", got, item.want)
		}
	}
}
