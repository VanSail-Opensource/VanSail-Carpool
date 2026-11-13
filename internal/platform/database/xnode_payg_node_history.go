package database

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// Reads are owner scoped and bounded independently of the current PAYG catalog.
// Historical node IDs survive node removal; their names fall back to the ID.
func (s *Store) XNodePAYGNodeHistory(ctx context.Context, user string, from, to time.Time) ([]xnode.PAYGNodeUsageHour, bool, error) {
	if to.Before(from) || to.Sub(from) > xnode.PAYGWindow+time.Minute {
		return nil, false, xnode.ErrInvalid
	}
	lower, upper := nativeUsageTimeBounds(from, to)
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT node_id FROM xnode_payg_node_minutes WHERE user_id=$1 AND minute>=$2 AND minute<=$3 ORDER BY node_id LIMIT $4`, user, lower, upper, xnode.MaxPAYGHistoryNodes+1)
	if err != nil {
		return nil, false, err
	}
	var nodes []string
	for rows.Next() {
		var node string
		if err = rows.Scan(&node); err != nil {
			rows.Close()
			return nil, false, err
		}
		nodes = append(nodes, node)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, false, err
	}
	truncated := len(nodes) > xnode.MaxPAYGHistoryNodes
	nodes = nodes[:min(len(nodes), xnode.MaxPAYGHistoryNodes)]
	result := []xnode.PAYGNodeUsageHour{}
	if len(nodes) == 0 {
		return result, false, nil
	}
	rows, err = s.db.QueryContext(ctx, `SELECT h.node_id,COALESCE(n.name,h.node_id),COALESCE(n.country_code,''),date_trunc('hour',h.minute,'UTC'),SUM(h.upload_bytes)::text,SUM(h.download_bytes)::text FROM xnode_payg_node_minutes h LEFT JOIN xnode_nodes n ON n.id=h.node_id WHERE h.user_id=$1 AND h.node_id=ANY($2::text[]) AND h.minute>=$3 AND h.minute<=$4 GROUP BY h.node_id,n.name,n.country_code,date_trunc('hour',h.minute,'UTC') ORDER BY h.node_id,date_trunc('hour',h.minute,'UTC') LIMIT $5`, user, nodes, lower, upper, xnode.MaxPAYGHistoryNodes*73)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var row xnode.PAYGNodeUsageHour
		if err = rows.Scan(&row.NodeID, &row.Name, &row.CountryCode, &row.Hour, &row.UploadBytes, &row.DownloadBytes); err != nil {
			return nil, false, err
		}
		result = append(result, row)
	}
	return result, truncated, rows.Err()
}
