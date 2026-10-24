package database

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (s *Store) XNodeIdentityByService(ctx context.Context, name string) (xnode.Identity, error) {
	return scanXNodeIdentity(s.db.QueryRowContext(ctx, xnodeIdentitySelect+` WHERE p.service_name=?`, name))
}

// Frequent activity reconciliation reads identifiers only, never the fleet's
// encrypted credentials or subscription material.
func (s *Store) XNodePrincipalOwners(ctx context.Context) (map[int]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT i.proxy_user_id,COALESCE(i.user_id,a.user_id,'') FROM xnode_identities i LEFT JOIN xnode_payg_accounts a ON a.proxy_user_id=i.proxy_user_id WHERE i.user_id IS NOT NULL OR a.user_id IS NOT NULL`)
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

func (s *Store) XNodeIdentitiesForNode(ctx context.Context, node string) ([]xnode.Identity, error) {
	rows, err := s.db.QueryContext(ctx, xnodeIdentitySelect+` WHERE p.service_name IS NULL OR p.service_name NOT LIKE 'member-payg-%' OR EXISTS(SELECT 1 FROM xnode_payg_accounts a WHERE a.proxy_user_id=p.proxy_user_id AND a.node_id=?) ORDER BY p.proxy_user_id`, node)
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
