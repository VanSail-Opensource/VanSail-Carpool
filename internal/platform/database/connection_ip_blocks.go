package database

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connections"
)

const ipBlockSelect = `SELECT id,user_id,node_uuid,ip_digest,sealed_ip,status,
	block_operation_id,unblock_operation_id,expiry_job_id,created_at,updated_at,expires_at
	FROM connection_ip_blocks`

// ConnectionIPBlockByOperation loads the encrypted target for a worker.
func (s *Store) ConnectionIPBlockByOperation(ctx context.Context, operationID string) (connections.IPBlockRecord, error) {
	return scanIPBlock(s.db.QueryRowContext(ctx, ipBlockSelect+
		` WHERE block_operation_id=$1 OR unblock_operation_id=$2`, operationID, operationID))
}

// ConnectionIPBlockForUser enforces owner-scoped lookup.
func (s *Store) ConnectionIPBlockForUser(ctx context.Context, blockID, userID string) (connections.IPBlockRecord, error) {
	return scanIPBlock(s.db.QueryRowContext(ctx, ipBlockSelect+` WHERE id=$1 AND user_id=$2`, blockID, userID))
}

// ConnectionIPBlockByID loads one active row for scheduled cleanup.
func (s *Store) ConnectionIPBlockByID(ctx context.Context, blockID string) (connections.IPBlockRecord, error) {
	return scanIPBlock(s.db.QueryRowContext(ctx, ipBlockSelect+` WHERE id=$1`, blockID))
}
