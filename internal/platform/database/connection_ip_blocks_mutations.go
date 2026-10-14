package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connections"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// ListConnectionIPBlocksForUser returns active encrypted rows in expiry order.
func (s *Store) ListConnectionIPBlocksForUser(ctx context.Context, userID string) ([]connections.IPBlockRecord, error) {
	rows, err := s.db.QueryContext(ctx, ipBlockSelect+` WHERE user_id=? ORDER BY expires_at,id`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	blocks := make([]connections.IPBlockRecord, 0)
	for rows.Next() {
		block, scanErr := scanIPBlock(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		blocks = append(blocks, block)
	}
	return blocks, rows.Err()
}

// BeginConnectionIPUnblock atomically marks the row and queues the command.
func (s *Store) BeginConnectionIPUnblock(ctx context.Context, blockID, ownerID string,
	command providerops.CreateInput, now time.Time) (providerops.Operation, bool, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return providerops.Operation{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	operation, replayed, err := createProviderOperationTx(ctx, tx, command, now.UTC())
	if err != nil {
		return providerops.Operation{}, false, err
	}
	if replayed {
		return operation, true, tx.Commit()
	}
	var existing sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT unblock_operation_id FROM connection_ip_blocks WHERE id=? AND user_id=?`,
		blockID, ownerID).Scan(&existing); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return providerops.Operation{}, false, ErrNotFound
		}
		return providerops.Operation{}, false, err
	}
	if existing.Valid {
		return providerops.Operation{}, false, ErrConflict
	}
	result, err := tx.ExecContext(ctx, `UPDATE connection_ip_blocks SET status='unblocking',unblock_operation_id=?,updated_at=?
		WHERE id=? AND user_id=? AND unblock_operation_id IS NULL`, operation.Receipt.ID, stamp(now), blockID, ownerID)
	if err != nil {
		return providerops.Operation{}, false, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return providerops.Operation{}, false, ErrConflict
	}
	return operation, replayed, tx.Commit()
}

func scanIPBlock(row rowScanner) (connections.IPBlockRecord, error) {
	var block connections.IPBlockRecord
	var blockOperation, unblockOperation sql.NullString
	var created, updated, expires string
	err := row.Scan(&block.ID, &block.UserID, &block.NodeUUID, &block.IPDigest, &block.SealedIP, &block.Status,
		&blockOperation, &unblockOperation, &block.ExpiryJobID, &created, &updated, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return connections.IPBlockRecord{}, connections.ErrIPBlockNotFound
	}
	if err != nil {
		return connections.IPBlockRecord{}, err
	}
	block.BlockOperationID, block.UnblockOperationID = blockOperation.String, unblockOperation.String
	if block.CreatedAt, err = parseStamp(created); err != nil {
		return connections.IPBlockRecord{}, err
	}
	if block.UpdatedAt, err = parseStamp(updated); err != nil {
		return connections.IPBlockRecord{}, err
	}
	block.ExpiresAt, err = parseStamp(expires)
	return block, err
}
