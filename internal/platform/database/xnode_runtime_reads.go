package database

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

const paygFundingPredicate = ` WHERE a.node_id=$1 AND b.txb_minor>=0
 AND (b.txb_minor>=` + paygFundingPrice + ` OR a.credit_bytes>0)`

// XNodePAYGFunding reads current authorization inputs without scanning periods.
// The account's validated period FK preserves existence; no counters are exposed.
func (s *Store) XNodePAYGFunding(ctx context.Context, node string) ([]xnode.PAYGFunding, error) {
	accounts, err := s.xnodePAYGAccounts(ctx, paygFundingSelect+paygFundingPredicate, []any{node})
	if err != nil {
		return nil, err
	}
	out := make([]xnode.PAYGFunding, 0, len(accounts))
	for _, account := range accounts {
		out = append(out, account.Funding())
	}
	return out, nil
}

// XNodePAYGFundedAccounts reads current funding candidates, never cached balances.
// Expiry stays in Go to preserve exact nanosecond timestamps on both backends.
func (s *Store) XNodePAYGFundedAccounts(ctx context.Context, node string) ([]xnode.PAYGAccount, error) {
	return s.xnodePAYGAccounts(ctx, paygSelect+paygFundingPredicate, []any{node})
}

// XNodePAYGConnectedAccounts limits observation reads to live positive counts.
// Chunks bound SQL parameters even when a node reports a large connection map.
func (s *Store) XNodePAYGConnectedAccounts(ctx context.Context, node string, ids []int64) ([]xnode.PAYGAccount, error) {
	out := []xnode.PAYGAccount{}
	for start := 0; start < len(ids); start += 128 {
		batch := ids[start:min(start+128, len(ids))]
		args := make([]any, 1, len(batch)+1)
		args[0] = node
		for _, id := range batch {
			args = append(args, id)
		}
		query := paygSelect + ` WHERE a.node_id=$1 AND a.proxy_user_id IN (` + placeholders(len(batch), 2) + `)`
		items, err := s.xnodePAYGAccounts(ctx, query, args)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
	}
	return out, nil
}

// XNodeCadenceIdentities excludes periods which cannot have a scheduled reset.
// Due times and recovery eligibility are still checked by the accounting service.
func (s *Store) XNodeCadenceIdentities(ctx context.Context) ([]xnode.Identity, error) {
	rows, err := s.db.QueryContext(ctx, xnodeIdentitySelect+` WHERE EXISTS (
	 SELECT 1 FROM xnode_periods period WHERE period.id=p.current_period_id
	 AND period.strategy NOT IN ('PAYG','NO_RESET') AND period.status<>'review') ORDER BY p.proxy_user_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []xnode.Identity{}
	for rows.Next() {
		item, err := scanXNodeIdentity(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
