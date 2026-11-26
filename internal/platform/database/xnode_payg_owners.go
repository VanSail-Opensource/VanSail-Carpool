package database

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (s *Store) XNodeIdentityByService(ctx context.Context, name string) (xnode.Identity, error) {
	return scanXNodeIdentity(s.db.QueryRowContext(ctx, xnodeIdentitySelect+` WHERE p.service_name=$1`, name))
}

// Frequent activity reconciliation reads identifiers only, never the fleet's
// encrypted credentials or subscription material.
func (s *Store) XNodePrincipalOwners(ctx context.Context) (map[int]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT i.proxy_user_id,COALESCE(i.user_id,a.user_id,e.owner_id,'') FROM xnode_identities i LEFT JOIN xnode_payg_accounts a ON a.proxy_user_id=i.proxy_user_id LEFT JOIN hourly_enrollments e ON e.proxy_user_id=i.proxy_user_id WHERE i.user_id IS NOT NULL OR a.user_id IS NOT NULL OR e.owner_id IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]string{}
	for rows.Next() {
		var id int
		var user string
		if err = rows.Scan(&id, &user); err != nil {
			return nil, err
		}
		out[id] = user
	}
	return out, rows.Err()
}

// XNodeIdentitiesForNode resolves authoritative owners through unique joins.
// Keep each hourly group singular even when a node exposes several inbounds.
func (s *Store) XNodeIdentitiesForNode(ctx context.Context, node string) ([]xnode.Identity, error) {
	rows, err := s.db.QueryContext(ctx, xnodeIdentityHead+`COALESCE(alias.old_proxy_id,0),COALESCE(payg.user_id,''),COALESCE(hourly.owner_id,''),p.first_synced_at
 FROM xnode_identities p LEFT JOIN users u ON u.id=p.user_id
 LEFT JOIN member_identity_aliases alias ON alias.proxy_id=p.proxy_user_id AND alias.old_proxy_id<>alias.proxy_id
 LEFT JOIN xnode_payg_accounts payg ON payg.proxy_user_id=p.proxy_user_id
 LEFT JOIN hourly_enrollments hourly ON hourly.proxy_user_id=p.proxy_user_id
 LEFT JOIN (SELECT DISTINCT group_id FROM xnode_group_bindings WHERE node_id=$1) hourly_group ON hourly_group.group_id=hourly.group_id
 WHERE p.service_name IS NULL OR (p.service_name NOT ILIKE 'member-payg-%' AND p.service_name NOT ILIKE 'member-hourly-%')
 OR payg.node_id=$1 OR hourly_group.group_id IS NOT NULL ORDER BY p.proxy_user_id`, node)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []xnode.Identity{}
	for rows.Next() {
		i, e := scanXNodeIdentity(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, i)
	}
	return out, rows.Err()
}
