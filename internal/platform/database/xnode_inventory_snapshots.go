package database

import (
	"context"
	"database/sql"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func xnodeGroupTx(ctx context.Context, tx *sql.Tx, id string) (xnode.Group, error) {
	var item xnode.Group
	var enabled int
	if err := tx.QueryRowContext(ctx, `SELECT id,name,enabled FROM xnode_groups WHERE id=$1`, id).Scan(&item.ID, &item.Name, &enabled); err != nil {
		return item, err
	}
	item.Enabled = enabled == 1
	item.Bindings = []xnode.GroupBinding{}
	rows, err := tx.QueryContext(ctx, `SELECT group_id,node_id,inbound_tag FROM xnode_group_bindings WHERE group_id=$1 ORDER BY node_id,inbound_tag`, id)
	if err != nil {
		return item, err
	}
	defer rows.Close()
	for rows.Next() {
		var binding xnode.GroupBinding
		if err = rows.Scan(&binding.GroupID, &binding.NodeID, &binding.InboundTag); err != nil {
			return item, err
		}
		item.Bindings = append(item.Bindings, binding)
	}
	return item, rows.Err()
}

func xnodeEndpointTx(ctx context.Context, tx *sql.Tx, id string) (xnode.Endpoint, error) {
	var item xnode.Endpoint
	var enabled, hidden int
	err := tx.QueryRowContext(ctx, `SELECT id,node_id,inbound_tag,name,address,port,enabled,hidden,overrides_cipher FROM xnode_endpoints WHERE id=$1`, id).Scan(&item.ID, &item.NodeID, &item.InboundTag, &item.Name, &item.Address, &item.Port, &enabled, &hidden, &item.OverridesCipher)
	item.Enabled, item.Hidden = enabled == 1, hidden == 1
	return item, err
}

func xnodeCustomizationTx(ctx context.Context, tx *sql.Tx, proxyID int64, nodeID string) (xnode.Customization, error) {
	var item xnode.Customization
	err := tx.QueryRowContext(ctx, `SELECT proxy_user_id,COALESCE(user_id,''),node_id,outbounds_cipher,routing_cipher,revision FROM xnode_customizations WHERE proxy_user_id=$1 AND node_id=$2`, proxyID, nodeID).Scan(&item.ProxyUserID, &item.UserID, &item.NodeID, &item.OutboundsCipher, &item.RoutingCipher, &item.Revision)
	return item, err
}
