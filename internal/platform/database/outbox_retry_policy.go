package database

import (
	"context"
	"database/sql"
	"time"
)

// Desired-state routing retries are safe at a short interval; the durable phase
// and receipt still fence completion. Monetary/provider commands retain their
// existing policy because an uncertain external effect must not be replayed.
func outboxRetryDelayTx(ctx context.Context, tx *sql.Tx, jobID string, attempts int) (time.Duration, error) {
	var kind, operationKind string
	err := tx.QueryRowContext(ctx, `SELECT j.kind,COALESCE(o.kind,'') FROM outbox_jobs j
		LEFT JOIN provider_operations o ON j.kind='provider_operation'
		AND o.id=(CASE ((CASE WHEN COALESCE((j.payload) IS JSON,false) THEN j.payload ELSE '{}' END)::jsonb #> '{operationId}')::text WHEN 'true' THEN '1' WHEN 'false' THEN '0' ELSE (CASE WHEN COALESCE((j.payload) IS JSON,false) THEN j.payload ELSE '{}' END)::jsonb #>> '{operationId}' END)
		WHERE j.id=$1`, jobID).Scan(&kind, &operationKind)
	if err != nil {
		return 0, err
	}
	if kind == "xnode_firewall" {
		return 5 * time.Second << min(max(attempts-1, 0), 4), nil
	}
	if kind == "provider_operation" && (operationKind == "xnode_routing" || operationKind == "xnode_diy" || operationKind == "xnode_ddns_update") {
		return min(5*time.Second<<min(max(attempts-1, 0), 3), 30*time.Second), nil
	}
	return time.Minute << min(max(attempts-1, 0), 8), nil
}
