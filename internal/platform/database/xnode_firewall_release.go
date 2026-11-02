package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

type firewallReleaseEvidence struct {
	Prefix     string `json:"prefix"`
	Generation string `json:"generation"`
	Version    uint64 `json:"version,string"`
}

// Release changes only one exact active generation. Paid/admin exemptions and
// detection evidence remain intact; no exemption is created for zero hours.
func (s *Store) releaseAdminFirewallBan(ctx context.Context, actor string, input xnode.AdminFirewallExemption, now time.Time) (model.OperationReceipt, error) {
	var receipt model.OperationReceipt
	err := s.firewallWrite(ctx, func(tx *sql.Tx) error {
		body, err := json.Marshal([]any{input.Prefix, input.Hours, input.Reason, input.Generation})
		if err != nil {
			return err
		}
		digest := sha256.Sum256(body)
		op, replayed, err := createProviderOperationTx(ctx, tx, providerops.CreateInput{ActorUserID: actor, Kind: xnode.FirewallExemptionOperation, IdempotencyKey: input.Key, RequestFingerprint: hex.EncodeToString(digest[:]), Items: []providerops.ItemInput{{Key: "release", TargetType: "firewall_ban_release", TargetID: input.Generation}}}, now)
		if err != nil {
			return err
		}
		receipt = op.Receipt
		if replayed {
			return nil
		}
		var found bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM xnode_firewall_bans WHERE prefix=$1 AND generation=$2 AND released_version=0)`, input.Prefix, input.Generation).Scan(&found); err != nil {
			return err
		}
		if !found {
			return xnode.ErrConflict
		}
		version, err := firewallVersion(ctx, tx)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE xnode_firewall_bans SET released_version=$1,released_at=$2 WHERE prefix=$3 AND generation=$4 AND released_version=0`, version, stamp(now), input.Prefix, input.Generation); err != nil {
			return err
		}
		evidence, err := json.Marshal(firewallReleaseEvidence{Prefix: input.Prefix, Generation: input.Generation, Version: version})
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE provider_operation_items SET result_json=$1 WHERE operation_id=$2 AND item_key='release'`, string(evidence), receipt.ID); err != nil {
			return err
		}
		audit, err := ids.New()
		if err != nil {
			return err
		}
		detail, err := json.Marshal(map[string]any{"prefix": input.Prefix, "generation": input.Generation, "hours": 0, "reason": input.Reason, "operationId": receipt.ID})
		if err != nil {
			return err
		}
		if err = insertAuditTx(ctx, tx, audit, &actor, "xnode_firewall_unban", "firewall_ban", input.Generation, string(detail), now); err != nil {
			return err
		}
		return queueFirewallFleetTx(ctx, tx, now)
	})
	return receipt, err
}

func finalizeFirewallReleaseTx(ctx context.Context, tx *sql.Tx, operation string, now time.Time) (bool, bool, error) {
	var status, raw string
	err := tx.QueryRowContext(ctx, `SELECT operation.status,item.result_json FROM provider_operations operation JOIN provider_operation_items item ON item.operation_id=operation.id AND item.item_key='release' AND item.target_type='firewall_ban_release' WHERE operation.id=$1 AND operation.kind=$2`, operation, xnode.FirewallExemptionOperation).Scan(&status, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, nil
	}
	if err != nil {
		return true, false, err
	}
	if providerops.Terminal(providerops.Status(status)) {
		return true, true, nil
	}
	var evidence firewallReleaseEvidence
	if json.Unmarshal([]byte(raw), &evidence) != nil || evidence.Version == 0 {
		return true, false, xnode.ErrInvalid
	}
	var pending bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM xnode_firewall_members WHERE active=1 AND (applied<$1 OR force_sync<>0))`, evidence.Version).Scan(&pending); err != nil {
		return true, false, err
	}
	if pending {
		return true, false, nil
	}
	if _, err = tx.ExecContext(ctx, `UPDATE provider_operation_items SET status='succeeded',provider_reference=$1,completed_at=$2,updated_at=$3 WHERE operation_id=$4 AND item_key='release'`, evidence.Generation, stamp(now), stamp(now), operation); err != nil {
		return true, false, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE provider_operations SET status='succeeded',provider_reference=$1,result_json=$2,completed_at=$3,updated_at=$4 WHERE id=$5 AND status IN('queued','processing')`, evidence.Generation, raw, stamp(now), stamp(now), operation)
	return true, err == nil, err
}
