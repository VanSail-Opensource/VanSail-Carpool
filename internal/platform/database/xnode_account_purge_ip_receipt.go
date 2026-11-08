package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// RecordAccountPurgeIPClear preserves only scope/receipt metadata, never the
// plaintext source address. The encrypted row survives until final account purge.
func (s *Store) RecordAccountPurgeIPClear(ctx context.Context, target model.AccountPurge, jobID string, proof xnode.AccountPurgeIPControl) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	_, allowed, err := s.accountNativePurgeGuardTx(ctx, tx, target, jobID)
	if err != nil || !allowed {
		return false, err
	}
	quiet, err := accountPurgeIPMutationsQuiescentTx(ctx, tx, target.UserID)
	if err != nil {
		return false, err
	}
	if !quiet {
		return false, xnode.ErrAccessAccountingPending
	}
	block, err := scanIPBlock(tx.QueryRowContext(ctx, ipBlockSelect+` WHERE id=$1 AND user_id=$2 FOR UPDATE`, proof.Block.ID, target.UserID))
	if err != nil {
		return false, err
	}
	if block.NodeUUID != proof.Block.NodeUUID || block.IPDigest != proof.Block.IPDigest || block.SealedIP != proof.Block.SealedIP || !block.ExpiresAt.Equal(proof.Block.ExpiresAt) || block.BlockOperationID != proof.Block.BlockOperationID {
		return false, xnode.ErrConflict
	}
	var agent string
	if err = tx.QueryRowContext(ctx, `SELECT agent_id FROM xnode_nodes WHERE id=$1 FOR KEY SHARE`, block.NodeUUID).Scan(&agent); err != nil {
		return false, err
	}
	ids, err := accountPurgeIPPrincipalsTx(ctx, tx, target)
	if err != nil {
		return false, err
	}
	if proof.AgentID == "" || proof.AgentID != agent || len(ids) == 0 || !slices.Equal(ids, proof.Principals) {
		return false, xnode.ErrConflict
	}
	detail, err := json.Marshal(map[string]any{
		"jobId": jobID, "generation": strconv.FormatInt(target.Generation, 10), "ownerId": target.UserID,
		"nodeId": block.NodeUUID, "agentId": agent, "principals": ids,
		"ipDigest": block.IPDigest, "expiresAt": stamp(block.ExpiresAt), "proof": "positive_subject_unblock",
	})
	if err != nil {
		return false, err
	}
	id := "account-purge-ip:" + jobID + ":" + block.ID
	var existing string
	err = tx.QueryRowContext(ctx, `SELECT detail FROM audit_events WHERE id=$1`, id).Scan(&existing)
	if err == nil && existing != string(detail) {
		return false, xnode.ErrConflict
	}
	if errors.Is(err, sql.ErrNoRows) {
		err = insertAuditTx(ctx, tx, id, nil, "account.purge_ip_cleared", "connection_ip_block", block.ID, string(detail), time.Now().UTC())
	}
	if err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func accountPurgeIPMutationsQuiescentTx(ctx context.Context, tx *sql.Tx, owner string) (bool, error) {
	// The common guard already locks/fences nonterminal provider operations.
	// Existence queries avoid loading arbitrarily many immutable old receipts.
	var pending bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM provider_operations o WHERE o.owner_user_id=$1
 AND o.kind='connection_block' AND (o.status IN('queued','processing','pending_review','partial')
 OR EXISTS(SELECT 1 FROM provider_operation_items i WHERE i.operation_id=o.id AND i.status IN('queued','processing','pending_review'))))`, owner).Scan(&pending)
	return !pending, err
}
