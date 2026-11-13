package database

import (
	"context"
	"crypto/sha256"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (s *Store) retryAttempt_SaveAdminXNodeNodeDDNS(ctx context.Context, actor string, node xnode.Node, bindings map[string]xnode.DDNSHostBinding) (xnode.Node, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return xnode.Node{}, err
	}
	defer tx.Rollback()
	if err = xnodeAdminActorTx(ctx, tx, actor); err != nil {
		return xnode.Node{}, err
	}
	if err = saveXNodeNodeDDNSTx(ctx, tx, node, bindings); err != nil {
		return xnode.Node{}, err
	}
	node, err = scanXNodeNode(tx.QueryRowContext(ctx, xnodeNodeSelect+` WHERE id=$1`, node.ID))
	if err != nil {
		return xnode.Node{}, err
	}
	if err = catalogAdminAuditTx(ctx, tx, actor, "xnode.node.save", "xnode_node", node.ID, map[string]any{"enabled": node.Enabled, "ddnsJobCount": len(bindings)}, time.Now().UTC()); err != nil {
		return xnode.Node{}, err
	}
	if err = tx.Commit(); err != nil {
		return xnode.Node{}, err
	}
	return node, nil
}

func (s *Store) retryAttempt_DeleteAdminXNodeNode(ctx context.Context, actor, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = xnodeAdminActorTx(ctx, tx, actor); err != nil {
		return err
	}
	changed, err := deleteXNodeNodeTx(ctx, tx, id)
	if err != nil {
		return err
	}
	if changed {
		if err = catalogAdminAuditTx(ctx, tx, actor, "xnode.node.delete", "xnode_node", id, nil, time.Now().UTC()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) retryAttempt_RotateAdminXNodeToken(ctx context.Context, actor, id string, hash []byte) error {
	if len(hash) != sha256.Size {
		return xnode.ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = xnodeAdminActorTx(ctx, tx, actor); err != nil {
		return err
	}
	if err = rotateXNodeToken(ctx, tx, id, hash); err != nil {
		return err
	}
	if err = catalogAdminAuditTx(ctx, tx, actor, "xnode.node.token.rotate", "xnode_node", id, nil, time.Now().UTC()); err != nil {
		return err
	}
	return tx.Commit()
}
