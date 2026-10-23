package xnode

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
)

type diagnosticStorageError int

func (e diagnosticStorageError) Error() string { return "secret database detail" }
func (e diagnosticStorageError) Code() int     { return int(e) }

func TestFrameDiagnosticsExposeOnlyAllowlistedCauses(t *testing.T) {
	for _, item := range []struct {
		err  error
		want string
	}{
		{context.DeadlineExceeded, "deadline"}, {sql.ErrNoRows, "missing_record"},
		{ErrInvalid, "invalid_input"}, {ErrConflict, "state_conflict"},
		{diagnosticStorageError(2067), "storage_constraint"},
		{errors.New("url?token=private-value"), "internal_failure"},
	} {
		err := firewallEventFailure("XNODE_FIREWALL_EVENT_STORAGE_FAILED", fmt.Errorf("sensitive context: %w", item.err))
		if got := frameErrorCause(err); got != item.want {
			t.Fatalf("safe cause = %q, want %q", got, item.want)
		}
	}
}
