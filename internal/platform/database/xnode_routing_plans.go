package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"time"
)

const routingPlanSelect = `SELECT proxy_user_id,cipher,previous_cipher,legacy_cipher,revision,phase,operation_id,reason FROM xnode_routing_plans`

func (s *Store) XNodeUnmanagedRoutes(ctx context.Context, before time.Time) ([]xnode.Customization, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT c.proxy_user_id,c.user_id,c.node_id,c.outbounds_cipher,c.routing_cipher,c.revision FROM xnode_customizations c WHERE c.user_id IS NOT NULL AND julianday(c.updated_at)<julianday(?) AND NOT EXISTS(SELECT 1 FROM xnode_routing_plans p WHERE p.proxy_user_id=c.proxy_user_id) ORDER BY c.proxy_user_id,c.node_id`, stamp(before))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []xnode.Customization{}
	for rows.Next() {
		var c xnode.Customization
		if err = rows.Scan(&c.ProxyUserID, &c.UserID, &c.NodeID, &c.OutboundsCipher, &c.RoutingCipher, &c.Revision); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) ReplayXNodeRouting(ctx context.Context, user, key, fingerprint string) (model.OperationReceipt, bool, error) {
	op, err := scanProviderOperation(s.db.QueryRowContext(ctx, providerOperationSelect+` WHERE actor_user_id=? AND kind=? AND idempotency_key=?`, user, xnode.RoutingOperationKind, key))
	if errors.Is(err, ErrNotFound) {
		return model.OperationReceipt{}, false, nil
	}
	if err != nil {
		return model.OperationReceipt{}, false, err
	}
	if op.RequestFingerprint != fingerprint {
		return op.Receipt, false, xnode.ErrConflict
	}
	return op.Receipt, true, nil
}

func scanRoutingPlan(row interface{ Scan(...any) error }) (xnode.RoutingPlan, error) {
	var p xnode.RoutingPlan
	err := row.Scan(&p.ProxyUserID, &p.Cipher, &p.PreviousCipher, &p.LegacyCipher, &p.Revision, &p.Phase, &p.OperationID, &p.Reason)
	if errors.Is(err, sql.ErrNoRows) {
		return p, xnode.ErrNotFound
	}
	return p, err
}
func (s *Store) XNodeRoutingPlan(ctx context.Context, id int64) (xnode.RoutingPlan, error) {
	return scanRoutingPlan(s.db.QueryRowContext(ctx, routingPlanSelect+` WHERE proxy_user_id=?`, id))
}

func (s *Store) XNodeRoutingPlans(ctx context.Context) ([]xnode.RoutingPlan, error) {
	rows, err := s.db.QueryContext(ctx, routingPlanSelect+` ORDER BY proxy_user_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []xnode.RoutingPlan{}
	for rows.Next() {
		p, err := scanRoutingPlan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// QueueXNodeRouting commits the desired graph and its existing durable outbox
// operation together. Retries cannot create an unowned or unqueued generation.
func (s *Store) QueueXNodeRouting(ctx context.Context, p xnode.RoutingPlan, input providerops.CreateInput, now time.Time) (model.OperationReceipt, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	defer tx.Rollback()
	op, replayed, err := createProviderOperationTx(ctx, tx, input, now)
	if err != nil {
		return op.Receipt, err
	}
	if replayed {
		return op.Receipt, tx.Commit()
	}
	if err = queueXNodeRoutingTx(ctx, tx, p, input, op.Receipt.ID, now); err != nil {
		return op.Receipt, err
	}
	return op.Receipt, tx.Commit()
}
func (s *Store) AdvanceXNodeRouting(ctx context.Context, id int64, operation, from, to string, now time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	valid := to == "disabled" || from == "revoking" && to == "applying" || from == "applying" && to == "enabling" || from == "enabling" && to == "active"
	if !valid {
		return xnode.ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE xnode_routing_plans SET phase=?,reason=CASE WHEN ?='disabled' THEN 'routing_dependencies_changed' ELSE '' END,previous_cipher=CASE WHEN ?='active' THEN '' ELSE previous_cipher END,updated_at=? WHERE proxy_user_id=? AND operation_id=? AND phase=?`, to, to, to, stamp(now), id, operation, from)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n != 1 {
		return xnode.ErrConflict
	}
	if err != nil {
		return err
	}
	var actor string
	if err = tx.QueryRowContext(ctx, `SELECT actor_user_id FROM provider_operations WHERE id=?`, operation).Scan(&actor); err != nil {
		return err
	}
	reason := ""
	if to == "disabled" {
		reason = "routing_dependencies_changed"
	}
	if err = routingAuditTx(ctx, tx, actor, operation, to, reason, false, now); err != nil {
		return err
	}
	return tx.Commit()
}

func routingAuditTx(ctx context.Context, tx *sql.Tx, actor, operation, phase, reason string, archived bool, now time.Time) error {
	id, err := ids.New()
	if err != nil {
		return err
	}
	raw, err := json.Marshal(map[string]any{"phase": phase, "reason": reason, "legacyEvidenceRetained": archived})
	if err != nil {
		return err
	}
	return insertAuditTx(ctx, tx, id, &actor, "xnode.routing."+phase, "provider_operation", operation, string(raw), now)
}
