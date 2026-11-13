package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// Rate changes and journal commits serialize on the same cursor before reading
// or changing sequence prices. The transaction owns the relevant row locks.
func lockXNodeCursorTx(ctx context.Context, tx *sql.Tx, node, agent string) (uint64, string, error) {
	if _, err := tx.ExecContext(ctx, `INSERT INTO xnode_report_cursors(agent_id,node_id,last_sequence,last_digest,updated_at) VALUES($1,$2,0,'',$3) ON CONFLICT(agent_id) DO NOTHING`, agent, node, stamp(time.Now().UTC())); err != nil {
		return 0, "", err
	}
	var sequence uint64
	var digest, owner string
	err := tx.QueryRowContext(ctx, `SELECT last_sequence,last_digest,node_id FROM xnode_report_cursors WHERE agent_id=$1 FOR UPDATE`, agent).Scan(&sequence, &digest, &owner)
	if err == nil && owner != node {
		err = xnode.ErrTrafficAgentMismatch
	}
	return sequence, digest, err
}
