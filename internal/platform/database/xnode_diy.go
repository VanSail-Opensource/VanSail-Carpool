package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (s *Store) XNodeDIY(ctx context.Context, id string) (xnode.DIYDocument, error) {
	doc := xnode.DIYDocument{UserID: id}
	err := s.db.QueryRowContext(ctx, `SELECT revision,cipher FROM xnode_member_diy WHERE user_id=?`, id).Scan(&doc.Revision, &doc.Cipher)
	if errors.Is(err, sql.ErrNoRows) {
		return doc, nil
	}
	return doc, err
}

func (s *Store) ReplayXNodeDIY(ctx context.Context, user, key, fingerprint string) (model.OperationReceipt, bool, bool, error) {
	op, err := scanProviderOperation(s.db.QueryRowContext(ctx, providerOperationSelect+` WHERE actor_user_id=? AND kind=? AND idempotency_key=?`, user, xnode.DIYOperationKind, key))
	if errors.Is(err, ErrNotFound) {
		return model.OperationReceipt{}, false, false, nil
	}
	if err != nil {
		return model.OperationReceipt{}, false, false, err
	}
	if op.OwnerUserID != user || op.RequestFingerprint != fingerprint {
		return op.Receipt, false, false, xnode.ErrConflict
	}
	var routed bool
	err = s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM provider_operation_items WHERE operation_id=? AND target_type='xnode_user')`, op.Receipt.ID).Scan(&routed)
	return op.Receipt, true, routed, err
}

// SaveXNodeDIY commits client overrides and optional routing desired state under
// both revision fences, in the same transaction as the existing durable receipt.
func (s *Store) retryAttempt_SaveXNodeDIY(ctx context.Context, doc xnode.DIYDocument, routingRevision int64, plan *xnode.RoutingPlan, input providerops.CreateInput, now time.Time) (model.OperationReceipt, error) {
	if doc.UserID == "" || doc.UserID != input.OwnerUserID || doc.UserID != input.ActorUserID {
		return model.OperationReceipt{}, xnode.ErrNotFound
	}
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
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
	var owner bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=? AND id=?)`, doc.UserID, input.OwnerUserID).Scan(&owner); err != nil {
		return op.Receipt, err
	}
	if !owner {
		return op.Receipt, xnode.ErrNotFound
	}
	var current int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE((SELECT revision FROM xnode_member_diy WHERE user_id=?),0)`, doc.UserID).Scan(&current); err != nil {
		return op.Receipt, err
	}
	var currentRouting int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE((SELECT p.revision FROM xnode_routing_plans p JOIN xnode_identities i ON i.proxy_user_id=p.proxy_user_id WHERE i.user_id=?),0)`, doc.UserID).Scan(&currentRouting); err != nil {
		return op.Receipt, err
	}
	if current != doc.Revision || currentRouting != routingRevision || current >= 9007199254740991 {
		return op.Receipt, xnode.ErrConflict
	}
	if !doc.Unchanged {
		if err = chargeDIYTx(ctx, tx, doc.UserID, op.Receipt.ID, doc.Billing, now); err != nil {
			return op.Receipt, err
		}
	}
	if plan != nil {
		var planOwned bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM xnode_identities WHERE proxy_user_id=? AND user_id=?)`, plan.ProxyUserID, doc.UserID).Scan(&planOwned); err != nil {
			return op.Receipt, err
		}
		if !planOwned {
			return op.Receipt, xnode.ErrNotFound
		}
		if plan.Revision != routingRevision {
			return op.Receipt, xnode.ErrConflict
		}
		if err = queueXNodeRoutingTx(ctx, tx, *plan, input, op.Receipt.ID, now); err != nil {
			return op.Receipt, err
		}
	}
	if !doc.Unchanged {
		if _, err = tx.ExecContext(ctx, `INSERT INTO xnode_member_diy(user_id,revision,cipher) VALUES(?,?,?) ON CONFLICT(user_id) DO UPDATE SET revision=excluded.revision,cipher=excluded.cipher`, doc.UserID, current+1, doc.Cipher); err != nil {
			return op.Receipt, err
		}
	}
	if doc.Result != nil {
		raw, err := json.Marshal(map[string]any{"diySave": doc.Result, "whitelistChanged": doc.NativeChanged})
		if err != nil {
			return op.Receipt, err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE provider_operations SET result_json=? WHERE id=?`, string(raw), op.Receipt.ID); err != nil {
			return op.Receipt, err
		}
	}
	if doc.NativeChanged {
		if _, err = tx.ExecContext(ctx, `UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1`); err != nil {
			return op.Receipt, err
		}
	}
	if plan == nil && !doc.NativeChanged {
		if _, err = tx.ExecContext(ctx, `UPDATE provider_operation_items SET status='succeeded',completed_at=?,updated_at=? WHERE operation_id=?`, stamp(now), stamp(now), op.Receipt.ID); err != nil {
			return op.Receipt, err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE provider_operations SET status='succeeded',completed_at=?,updated_at=? WHERE id=?`, stamp(now), stamp(now), op.Receipt.ID); err != nil {
			return op.Receipt, err
		}
		op, err = scanProviderOperation(tx.QueryRowContext(ctx, providerOperationSelect+` WHERE id=?`, op.Receipt.ID))
		if err != nil {
			return op.Receipt, err
		}
	}
	return op.Receipt, tx.Commit()
}
