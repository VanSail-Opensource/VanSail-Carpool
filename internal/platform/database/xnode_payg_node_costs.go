package database

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// Exact recorded charges retain their account/node reference after retirement.
// Traffic rows are intentionally not joined: pre-purchased credit can produce a
// cost-only node, and combo usage cannot enter this owner-scoped PAYG query.
func (s *Store) XNodePAYGNodeCosts(ctx context.Context, user string, from, to time.Time) ([]xnode.PAYGNodeCost, bool, error) {
	if to.Before(from) || to.Sub(from) > xnode.PAYGWindow+time.Minute {
		return nil, false, xnode.ErrInvalid
	}
	rows, err := s.db.QueryContext(ctx, `SELECT a.node_id,COALESCE(n.name,a.node_id),COALESCE(n.country_code,''),SUM(c.blocks::numeric*c.unit_price_minor::numeric)::text
 FROM xnode_payg_charges c JOIN ledger_entries l ON l.id=c.ledger_id JOIN xnode_payg_accounts a ON a.proxy_user_id=c.proxy_user_id
 LEFT JOIN xnode_nodes n ON n.id=a.node_id
 WHERE a.user_id=$1 AND l.user_id=$1 AND l.created_at::timestamptz>=$2 AND l.created_at::timestamptz<=$3
 GROUP BY a.node_id,n.name,n.country_code ORDER BY a.node_id LIMIT $4`, user, from, to, xnode.MaxPAYGHistoryNodes+1)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	costs := []xnode.PAYGNodeCost{}
	for rows.Next() {
		var cost xnode.PAYGNodeCost
		if err = rows.Scan(&cost.NodeID, &cost.Name, &cost.CountryCode, &cost.SpentMinor); err != nil {
			return nil, false, err
		}
		costs = append(costs, cost)
	}
	if err = rows.Err(); err != nil {
		return nil, false, err
	}
	truncated := len(costs) > xnode.MaxPAYGHistoryNodes
	return costs[:min(len(costs), xnode.MaxPAYGHistoryNodes)], truncated, nil
}
