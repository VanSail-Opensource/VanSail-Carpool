package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// Administrative changes take effect atomically; notification is durable queued
// follow-up work and never a second-person approval gate.
func pricingChangeNoticeTx(ctx context.Context, tx *sql.Tx, actor, command, scope, reason string, now time.Time) error {
	raw, err := json.Marshal(map[string]string{"actor": actor, "command": command, "scope": scope, "reason": reason})
	if err != nil {
		return err
	}
	return insertOutboxTx(ctx, tx, "pricing_change_notice", string(raw), now, now)
}
