package database

import (
	"context"
	"database/sql"
	"time"
)

// Rate changes and journal commits serialize on the same cursor before reading
// or changing sequence prices. SQLite callers already hold writeMu.
func lockXNodeCursorTx(ctx context.Context, tx *sql.Tx, node, agent string) (uint64, string, error) {
	if _, err := tx.ExecContext(ctx, `INSERT INTO xnode_report_cursors(agent_id,node_id,last_sequence,last_digest,updated_at) VALUES(?,?,0,'',?) ON CONFLICT(agent_id) DO NOTHING`, agent, node, stamp(time.Now().UTC())); err != nil {
		return 0, "", err
	}
	var sequence uint64
	var digest string
	err := tx.QueryRowContext(ctx, `SELECT last_sequence,last_digest FROM xnode_report_cursors WHERE agent_id=? /*pg:FOR UPDATE*/`, agent).Scan(&sequence, &digest)
	return sequence, digest, err
}
