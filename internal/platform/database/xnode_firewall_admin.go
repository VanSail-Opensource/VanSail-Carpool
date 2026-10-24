package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (s *Store) GrantAdminFirewallExemption(ctx context.Context, actor string, input xnode.AdminFirewallExemption, now time.Time) (model.OperationReceipt, error) {
	if input.Hours == 0 {
		return s.releaseAdminFirewallBan(ctx, actor, input, now)
	}
	var receipt model.OperationReceipt
	err := s.firewallWrite(ctx, func(tx *sql.Tx) error {
		grant, err := ids.New()
		if err != nil {
			return err
		}
		body, err := json.Marshal([]any{input.Prefix, input.Hours, input.Reason})
		if err != nil {
			return err
		}
		digest := sha256.Sum256(body)
		op, replayed, err := createProviderOperationTx(ctx, tx, providerops.CreateInput{ActorUserID: actor, Kind: xnode.FirewallExemptionOperation, IdempotencyKey: input.Key, RequestFingerprint: hex.EncodeToString(digest[:]), Items: []providerops.ItemInput{{Key: "grant", TargetType: "firewall_exemption", TargetID: grant}}}, now)
		if err != nil {
			return err
		}
		receipt = op.Receipt
		if replayed {
			return nil
		}
		version, err := firewallVersion(ctx, tx)
		if err != nil {
			return err
		}
		until := now.Add(time.Duration(input.Hours) * time.Hour)
		if _, err = tx.ExecContext(ctx, `INSERT INTO xnode_firewall_exemptions(id,user_id,idempotency_key,ip,price_minor,hours,until_at,created_at,issued_version,grant_kind) VALUES(?,?,?,?,0,?,?,?,?,'admin')`, grant, actor, input.Key, input.Prefix, input.Hours, stamp(until), stamp(now), version); err != nil {
			return err
		}
		audit, err := ids.New()
		if err != nil {
			return err
		}
		detail, _ := json.Marshal(map[string]any{"prefix": input.Prefix, "hours": input.Hours, "reason": input.Reason, "operationId": receipt.ID})
		if err = insertAuditTx(ctx, tx, audit, &actor, "xnode_firewall_exemption", "firewall_exemption", grant, string(detail), now); err != nil {
			return err
		}
		return queueFirewallFleetTx(ctx, tx, now)
	})
	return receipt, err
}

// Completion observes the existing fleet delivery clock; it sends no duplicate
// node command and never charges a balance or changes a member's paid grant.
func (s *Store) FinalizeAdminFirewallExemption(ctx context.Context, operation string, now time.Time) (bool, error) {
	done := false
	err := s.firewallWrite(ctx, func(tx *sql.Tx) error {
		var until, status, grant string
		if handled, complete, err := finalizeFirewallReleaseTx(ctx, tx, operation, now); handled || err != nil {
			done = complete
			return err
		}
		var version uint64
		err := tx.QueryRowContext(ctx, `SELECT exemption.id,exemption.until_at,exemption.issued_version,operation.status FROM provider_operations operation JOIN provider_operation_items item ON item.operation_id=operation.id AND item.item_key='grant' JOIN xnode_firewall_exemptions exemption ON exemption.id=item.target_id AND exemption.grant_kind='admin' WHERE operation.id=? AND operation.kind=?`, operation, xnode.FirewallExemptionOperation).Scan(&grant, &until, &version, &status)
		if err != nil {
			return err
		}
		if providerops.Terminal(providerops.Status(status)) {
			done = true
			return nil
		}
		deadline, err := parseStamp(until)
		if err != nil {
			return err
		}
		expired := !now.Before(deadline)
		var pending bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM xnode_firewall_members WHERE active=1 AND (applied<? OR force_sync<>0))`, version).Scan(&pending); err != nil {
			return err
		}
		if pending && !expired {
			return nil
		}
		status, errorCode := "succeeded", ""
		if expired {
			status, errorCode = "failed", "XNODE_EXEMPTION_EXPIRED"
		}
		result, _ := json.Marshal(map[string]string{"exemptionId": grant})
		if _, err = tx.ExecContext(ctx, `UPDATE provider_operation_items SET status=?,provider_reference=?,error_code=?,result_json=?,completed_at=?,updated_at=? WHERE operation_id=? AND item_key='grant'`, status, grant, errorCode, string(result), stamp(now), stamp(now), operation); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE provider_operations SET status=?,provider_reference=?,error_code=?,result_json=?,completed_at=?,updated_at=? WHERE id=? AND status IN ('queued','processing')`, status, grant, errorCode, string(result), stamp(now), stamp(now), operation)
		done = err == nil
		return err
	})
	return done, err
}
