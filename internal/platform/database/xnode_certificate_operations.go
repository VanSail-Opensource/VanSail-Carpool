package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// CreateXNodeCertificateOperation serializes one profile's intents across all
// administrators, preserving replay before refusing a new uncertain operation.
func (s *Store) CreateXNodeCertificateOperation(ctx context.Context, input providerops.CreateInput, profile, action string, revision uint64, now time.Time) (providerops.Operation, bool, error) {
	input.Items = slices.Clone(input.Items)
	input, err := providerops.NormalizeCreate(input)
	if err != nil || input.Kind != "xnode_command" || len(input.Items) != 1 || !wire.ValidName(profile) || action != "issue" && action != "renew" || revision == 0 {
		return providerops.Operation{}, false, xnode.ErrInvalid
	}
	if item := input.Items[0]; item.Key != "certificate:"+profile || item.TargetType != "xnode" {
		return providerops.Operation{}, false, xnode.ErrInvalid
	}
	for attempt := 0; ; attempt++ {
		operation, replayed, err := s.createXNodeCertificateOperation(ctx, input, profile, action, revision, now.UTC())
		if !s.retryTransaction(ctx, err, attempt) {
			if errors.Is(err, ErrConflict) {
				err = errors.Join(err, xnode.ErrConflict)
			}
			return operation, replayed, err
		}
	}
}

func (s *Store) createXNodeCertificateOperation(ctx context.Context, input providerops.CreateInput, profile, action string, revision uint64, now time.Time) (providerops.Operation, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return providerops.Operation{}, false, fmt.Errorf("begin certificate operation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	node := input.Items[0].TargetID
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "xnode.certificate:"+node+":"+profile); err != nil {
		return providerops.Operation{}, false, fmt.Errorf("lock certificate profile: %w", err)
	}
	operation, replayed, err := createProviderOperationTx(ctx, tx, input, now)
	if err != nil {
		return providerops.Operation{}, false, err
	}
	if !replayed {
		var blocked bool
		err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM provider_operations operation
			JOIN provider_operation_items item ON item.operation_id=operation.id
			WHERE operation.kind='xnode_command' AND operation.id<>$1
			AND operation.status IN('queued','processing','pending_review','partial')
			AND item.item_key=$2 AND item.target_type='xnode' AND item.target_id=$3)`,
			operation.Receipt.ID, input.Items[0].Key, node).Scan(&blocked)
		if err != nil {
			return providerops.Operation{}, false, fmt.Errorf("check active certificate operation: %w", err)
		}
		if blocked {
			return providerops.Operation{}, false, ErrConflict
		}
		raw, err := json.Marshal(map[string]any{"command": "certificates.run", "nodeId": node, "profile_id": profile, "action": action, "expected_revision": revision})
		if err != nil {
			return providerops.Operation{}, false, fmt.Errorf("encode certificate intent: %w", err)
		}
		if _, err = tx.ExecContext(ctx, `UPDATE provider_operations SET result_json=$1 WHERE id=$2`, string(raw), operation.Receipt.ID); err != nil {
			return providerops.Operation{}, false, fmt.Errorf("record certificate intent: %w", err)
		}
		operation.ResultJSON = string(raw)
	}
	if err = tx.Commit(); err != nil {
		return providerops.Operation{}, false, fmt.Errorf("commit certificate operation: %w", err)
	}
	return operation, replayed, nil
}

// LatestXNodeCertificateOperation recovers a profile's newest durable receipt and
// safe intent/result across administrators and browser devices.
func (s *Store) LatestXNodeCertificateOperation(ctx context.Context, node, profile string) (providerops.Operation, error) {
	if node == "" || !wire.ValidName(profile) {
		return providerops.Operation{}, xnode.ErrInvalid
	}
	operation, err := scanProviderOperation(s.db.QueryRowContext(ctx, providerOperationSelect+`
		WHERE kind='xnode_command' AND EXISTS(SELECT 1 FROM provider_operation_items item
		WHERE item.operation_id=provider_operations.id AND item.item_key=$1
		AND item.target_type='xnode' AND item.target_id=$2)
		ORDER BY `+purchaseTimestampOrderSQL("created_at")+` DESC,id DESC LIMIT 1`, "certificate:"+profile, node))
	if errors.Is(err, ErrNotFound) {
		return providerops.Operation{}, errors.Join(err, xnode.ErrNotFound)
	}
	if err != nil {
		return providerops.Operation{}, fmt.Errorf("load latest certificate operation: %w", err)
	}
	return operation, nil
}
