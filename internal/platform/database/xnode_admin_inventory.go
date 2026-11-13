package database

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (s *Store) retryAttempt_SaveAdminXNodeGroup(ctx context.Context, actor string, item xnode.Group) (xnode.Group, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return xnode.Group{}, err
	}
	defer tx.Rollback()
	if err = xnodeAdminActorTx(ctx, tx, actor); err != nil {
		return xnode.Group{}, err
	}
	if err = saveXNodeGroupTx(ctx, tx, item); err != nil {
		return xnode.Group{}, err
	}
	item, err = xnodeGroupTx(ctx, tx, item.ID)
	if err != nil {
		return xnode.Group{}, err
	}
	if err = catalogAdminAuditTx(ctx, tx, actor, "xnode.group.save", "xnode_group", item.ID, map[string]any{"enabled": item.Enabled, "bindingCount": len(item.Bindings)}, time.Now().UTC()); err != nil {
		return xnode.Group{}, err
	}
	if err = tx.Commit(); err != nil {
		return xnode.Group{}, err
	}
	return item, nil
}

func (s *Store) retryAttempt_DeleteAdminXNodeGroupAtVersion(ctx context.Context, actor, id string, version int64, endpoints []xnode.Endpoint) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = xnodeAdminActorTx(ctx, tx, actor); err != nil {
		return err
	}
	// Attribute the existing deterministic accounting-retention audit rather
	// than adding a second event for the same group deletion.
	if err = deleteXNodeGroupTx(ctx, tx, &actor, id, version, endpoints); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) retryAttempt_SaveAdminXNodeEndpoint(ctx context.Context, actor string, item xnode.Endpoint) (xnode.Endpoint, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return xnode.Endpoint{}, err
	}
	defer tx.Rollback()
	if err = xnodeAdminActorTx(ctx, tx, actor); err != nil {
		return xnode.Endpoint{}, err
	}
	if err = saveXNodeEndpoint(ctx, tx, item); err != nil {
		return xnode.Endpoint{}, err
	}
	stored, err := xnodeEndpointTx(ctx, tx, item.ID)
	if err != nil {
		return xnode.Endpoint{}, err
	}
	// These preview values were validated alongside the encrypted material.
	stored.Overrides, stored.Metadata = item.Overrides, item.Metadata
	if err = catalogAdminAuditTx(ctx, tx, actor, "xnode.endpoint.save", "xnode_endpoint", stored.ID, map[string]any{"nodeId": stored.NodeID, "enabled": stored.Enabled}, time.Now().UTC()); err != nil {
		return xnode.Endpoint{}, err
	}
	if err = tx.Commit(); err != nil {
		return xnode.Endpoint{}, err
	}
	return stored, nil
}

func (s *Store) retryAttempt_DeleteAdminXNodeEndpoint(ctx context.Context, actor, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = xnodeAdminActorTx(ctx, tx, actor); err != nil {
		return err
	}
	if err = deleteXNodeEndpoint(ctx, tx, id); err != nil {
		return err
	}
	if err = catalogAdminAuditTx(ctx, tx, actor, "xnode.endpoint.delete", "xnode_endpoint", id, nil, time.Now().UTC()); err != nil {
		return err
	}
	return tx.Commit()
}
