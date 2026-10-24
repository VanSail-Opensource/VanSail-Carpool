package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connections"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

const ipBlockSelect = `SELECT id,user_id,node_uuid,ip_digest,sealed_ip,status,
	block_operation_id,unblock_operation_id,expiry_job_id,created_at,updated_at,expires_at
	FROM connection_ip_blocks`

// BeginConnectionIPBlock atomically stores the encrypted target and both jobs.
func (s *Store) retryAttempt_BeginConnectionIPBlock(ctx context.Context, input connections.CreateIPBlockInput,
	command providerops.CreateInput, now time.Time) (connections.IPBlockRecord, providerops.Operation, bool, error) {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return connections.IPBlockRecord{}, providerops.Operation{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	operation, replayed, err := createProviderOperationTx(ctx, tx, command, now.UTC())
	if err != nil {
		return connections.IPBlockRecord{}, providerops.Operation{}, false, err
	}
	if replayed {
		block, loadErr := scanIPBlock(tx.QueryRowContext(ctx, ipBlockSelect+` WHERE block_operation_id=?`, operation.Receipt.ID))
		if loadErr != nil {
			return connections.IPBlockRecord{}, providerops.Operation{}, false, loadErr
		}
		return block, operation, true, tx.Commit()
	}
	blockID, err := ids.New()
	if err != nil {
		return connections.IPBlockRecord{}, providerops.Operation{}, false, err
	}
	payload, _ := json.Marshal(map[string]string{"blockId": blockID})
	expiryJobID, err := insertOutboxJobTx(ctx, tx, connections.BlockExpiryOutboxKind, string(payload), input.ExpiresAt.UTC(), now.UTC())
	if err != nil {
		return connections.IPBlockRecord{}, providerops.Operation{}, false, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO connection_ip_blocks(id,user_id,node_uuid,ip_digest,sealed_ip,status,
		block_operation_id,expiry_job_id,created_at,updated_at,expires_at) VALUES(?,?,?,?,?,'blocking',?,?,?,?,?)`,
		blockID, input.UserID, input.NodeUUID, input.IPDigest, input.SealedIP, operation.Receipt.ID,
		expiryJobID, stamp(now), stamp(now), stamp(input.ExpiresAt))
	if err != nil {
		if isUniqueViolation(err) {
			return connections.IPBlockRecord{}, providerops.Operation{}, false, ErrConflict
		}
		return connections.IPBlockRecord{}, providerops.Operation{}, false, fmt.Errorf("insert connection IP block: %w", err)
	}
	block, err := scanIPBlock(tx.QueryRowContext(ctx, ipBlockSelect+` WHERE id=?`, blockID))
	if err != nil {
		return connections.IPBlockRecord{}, providerops.Operation{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return connections.IPBlockRecord{}, providerops.Operation{}, false, err
	}
	return block, operation, false, nil
}

// ConnectionIPBlockByOperation loads the encrypted target for a worker.
func (s *Store) ConnectionIPBlockByOperation(ctx context.Context, operationID string) (connections.IPBlockRecord, error) {
	return scanIPBlock(s.db.QueryRowContext(ctx, ipBlockSelect+
		` WHERE block_operation_id=? OR unblock_operation_id=?`, operationID, operationID))
}

// ConnectionIPBlockForUser enforces owner-scoped lookup.
func (s *Store) ConnectionIPBlockForUser(ctx context.Context, blockID, userID string) (connections.IPBlockRecord, error) {
	return scanIPBlock(s.db.QueryRowContext(ctx, ipBlockSelect+` WHERE id=? AND user_id=?`, blockID, userID))
}

// ConnectionIPBlockByID loads one active row for scheduled cleanup.
func (s *Store) ConnectionIPBlockByID(ctx context.Context, blockID string) (connections.IPBlockRecord, error) {
	return scanIPBlock(s.db.QueryRowContext(ctx, ipBlockSelect+` WHERE id=?`, blockID))
}
