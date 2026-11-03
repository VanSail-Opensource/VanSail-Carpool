package database

import (
	"context"
	"database/sql"
	"maps"
	"slices"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func prepareTrafficBatch(ctx context.Context, tx *sql.Tx, items []xnode.TrafficCommit) (*trafficBatchState, error) {
	b := &trafficBatchState{nodes: map[string]trafficNodeValue{}, cursors: map[trafficCursorKey]*trafficCursorValue{}, rates: map[trafficRateKey]int64{}, payg: map[int64]bool{}}
	ids := map[string]bool{}
	for _, item := range items {
		ids[item.NodeID] = true
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,COALESCE(agent_id,''),multiplier_fp FROM xnode_nodes WHERE id=ANY($1::text[]) ORDER BY id FOR KEY SHARE`, slices.Sorted(maps.Keys(ids)))
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id string
		var v trafficNodeValue
		if err = rows.Scan(&id, &v.agent, &v.multiplier); err != nil {
			rows.Close()
			return nil, err
		}
		b.nodes[id] = v
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	valid := make([]xnode.TrafficCommit, 0, len(items))
	for _, item := range items {
		if node, exists := b.nodes[item.NodeID]; exists && node.agent == item.AgentID {
			valid = append(valid, item)
		}
	}
	// Stable global lock order: nodes, journal cursors, wallet owners, periods.
	if err = loadTrafficCursors(ctx, tx, b, valid); err != nil {
		return nil, err
	}
	billable := make([]xnode.TrafficCommit, 0, len(valid))
	for _, item := range valid {
		cursor := b.cursors[trafficCursorKey{item.AgentID, item.Report.StreamID}]
		if cursor != nil && !cursor.closed && item.Sequence > cursor.sequence {
			billable = append(billable, item)
		}
	}
	if err = loadTrafficRates(ctx, tx, b, billable); err != nil {
		return nil, err
	}
	if err = lockTrafficWallets(ctx, tx, b, billable); err != nil {
		return nil, err
	}
	b.native, err = loadNativeTraffic(ctx, tx, billable)
	return b, err
}

func lockTrafficWallets(ctx context.Context, tx *sql.Tx, b *trafficBatchState, items []xnode.TrafficCommit) error {
	proxies := map[int64]bool{}
	for _, item := range items {
		for _, sample := range item.Report.Samples {
			proxies[int64(sample.UserID)] = true
		}
	}
	ids := slices.Sorted(maps.Keys(proxies))
	owners := map[string]bool{}
	for start := 0; start < len(ids); start += trafficSQLRows {
		rows, err := tx.QueryContext(ctx, `SELECT proxy_user_id,user_id FROM xnode_payg_accounts WHERE proxy_user_id=ANY($1::bigint[]) ORDER BY proxy_user_id`, ids[start:min(start+trafficSQLRows, len(ids))])
		if err != nil {
			return err
		}
		for rows.Next() {
			var id int64
			var owner string
			if err = rows.Scan(&id, &owner); err != nil {
				rows.Close()
				return err
			}
			b.payg[id], owners[owner] = true, true
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	users := slices.Sorted(maps.Keys(owners))
	for start := 0; start < len(users); start += trafficSQLRows {
		chunk := users[start:min(start+trafficSQLRows, len(users))]
		rows, err := tx.QueryContext(ctx, `SELECT user_id FROM balances WHERE user_id=ANY($1::text[]) ORDER BY user_id FOR UPDATE`, chunk)
		if err != nil {
			return err
		}
		count := 0
		for rows.Next() {
			count++
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if count != len(chunk) {
			return ErrNotFound
		}
	}
	return nil
}
