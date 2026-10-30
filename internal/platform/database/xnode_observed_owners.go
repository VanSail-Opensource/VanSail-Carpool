package database

import (
	"context"
	"encoding/json"
)

// XNodeObservedPrincipalOwners returns every sibling of an observed owner so
// idle principals still receive a global block, without scanning unrelated users.
func (s *Store) XNodeObservedPrincipalOwners(ctx context.Context, ids []int) (map[int]string, error) {
	out := map[int]string{}
	if len(ids) == 0 {
		return out, nil
	}
	raw, err := json.Marshal(ids)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `WITH observed AS (SELECT CAST(value AS BIGINT) AS id FROM json_each(?)), owners AS (SELECT COALESCE(i.user_id,a.user_id,'') AS owner FROM xnode_identities i LEFT JOIN xnode_payg_accounts a ON a.proxy_user_id=i.proxy_user_id WHERE i.proxy_user_id IN (SELECT id FROM observed)) SELECT proxy_user_id,user_id FROM xnode_identities WHERE user_id IN (SELECT owner FROM owners WHERE owner<>'') UNION ALL SELECT proxy_user_id,user_id FROM xnode_payg_accounts WHERE user_id IN (SELECT owner FROM owners WHERE owner<>'')`, string(raw))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var owner string
		if err = rows.Scan(&id, &owner); err != nil {
			return nil, err
		}
		out[id] = owner
	}
	return out, rows.Err()
}
