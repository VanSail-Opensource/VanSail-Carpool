package database

import (
	"context"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (s *Store) retryAttempt_SaveAdminXNodeNamedService(ctx context.Context, actor string, item xnode.Identity, access xnode.ServiceAccess) (xnode.Identity, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return xnode.Identity{}, err
	}
	defer tx.Rollback()
	if err = xnodeAdminActorTx(ctx, tx, actor); err != nil {
		return xnode.Identity{}, err
	}
	if item.MemberOwnerID() != "" || item.ServiceName == "" {
		return xnode.Identity{}, xnode.ErrInvalid
	}
	item, err = saveXNodeNamedServiceTx(ctx, tx, item, access)
	if err != nil {
		return xnode.Identity{}, err
	}
	if err = catalogAdminAuditTx(ctx, tx, actor, "xnode.named_service.save", "xnode_identity", strconv.FormatInt(item.ProxyUserID, 10), map[string]any{"accessDisabled": item.AccessDisabled}, time.Now().UTC()); err != nil {
		return xnode.Identity{}, err
	}
	if err = tx.Commit(); err != nil {
		return xnode.Identity{}, err
	}
	return item, nil
}

func (s *Store) retryAttempt_SaveAdminXNodeCustomization(ctx context.Context, actor string, item xnode.Customization) (xnode.Customization, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return xnode.Customization{}, err
	}
	defer tx.Rollback()
	if err = xnodeAdminActorTx(ctx, tx, actor); err != nil {
		return xnode.Customization{}, err
	}
	identity, err := scanXNodeIdentity(tx.QueryRowContext(ctx, xnodeIdentitySelect+` WHERE p.proxy_user_id=$1 FOR SHARE OF p`, item.ProxyUserID))
	if err != nil {
		return xnode.Customization{}, err
	}
	// Member customizations use their managed, owner-scoped plan workflow.
	if identity.MemberOwnerID() != "" || identity.ServiceName == "" || item.UserID != "" {
		return xnode.Customization{}, xnode.ErrConflict
	}
	if err = saveXNodeCustomization(ctx, tx, item); err != nil {
		return xnode.Customization{}, err
	}
	stored, err := xnodeCustomizationTx(ctx, tx, item.ProxyUserID, item.NodeID)
	if err != nil {
		return xnode.Customization{}, err
	}
	stored.Outbounds, stored.Routing = item.Outbounds, item.Routing
	if err = catalogAdminAuditTx(ctx, tx, actor, "xnode.customization.save", "xnode_identity", strconv.FormatInt(stored.ProxyUserID, 10), map[string]any{"nodeId": stored.NodeID, "revision": stored.Revision}, time.Now().UTC()); err != nil {
		return xnode.Customization{}, err
	}
	if err = tx.Commit(); err != nil {
		return xnode.Customization{}, err
	}
	return stored, nil
}
