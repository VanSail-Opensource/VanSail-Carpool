package database

import (
	"cmp"
	"context"
	"database/sql"
	"maps"
	"slices"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func sortedTrafficCursors[V any](values map[trafficCursorKey]V) []trafficCursorKey {
	keys := slices.Collect(maps.Keys(values))
	slices.SortFunc(keys, func(a, b trafficCursorKey) int {
		return cmp.Or(cmp.Compare(a.agent, b.agent), cmp.Compare(a.stream, b.stream))
	})
	return keys
}

func loadTrafficCursors(ctx context.Context, tx *sql.Tx, b *trafficBatchState, items []xnode.TrafficCommit) error {
	keys := map[trafficCursorKey]string{}
	for _, item := range items {
		keys[trafficCursorKey{item.AgentID, item.Report.StreamID}] = item.NodeID
	}
	var agents, nodes, streamAgents, streams []string
	for _, key := range sortedTrafficCursors(keys) {
		if key.stream == "" {
			agents, nodes = append(agents, key.agent), append(nodes, keys[key])
		} else {
			streamAgents, streams = append(streamAgents, key.agent), append(streams, key.stream)
		}
	}
	// Legacy and new cursors have a fixed bank order. Rate changes take the node
	// lock first, so they cannot invert this order against a mixed fleet batch.
	if len(agents) > 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO xnode_report_cursors(agent_id,node_id,last_sequence,last_digest,updated_at) SELECT a.agent_id,a.node_id,0,'',$3 FROM unnest($1::text[],$2::text[]) AS a(agent_id,node_id) ON CONFLICT(agent_id) DO NOTHING`, agents, nodes, stamp(time.Now().UTC())); err != nil {
			return err
		}
		rows, err := tx.QueryContext(ctx, `SELECT agent_id,node_id,last_sequence,last_digest FROM xnode_report_cursors WHERE agent_id=ANY($1::text[]) ORDER BY agent_id FOR UPDATE`, agents)
		if err != nil {
			return err
		}
		for rows.Next() {
			var key trafficCursorKey
			v := &trafficCursorValue{}
			if err = rows.Scan(&key.agent, &v.node, &v.sequence, &v.digest); err != nil {
				rows.Close()
				return err
			}
			b.cursors[key] = v
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	if len(streams) > 0 {
		rows, err := tx.QueryContext(ctx, `SELECT s.agent_id,s.stream_id,s.node_id,s.last_sequence,s.last_digest,s.closed,s.has_gaps FROM xnode_traffic_streams s JOIN unnest($1::text[],$2::text[]) AS k(agent_id,stream_id) ON s.agent_id=k.agent_id AND s.stream_id=k.stream_id ORDER BY s.agent_id,s.stream_id FOR UPDATE OF s`, streamAgents, streams)
		if err != nil {
			return err
		}
		for rows.Next() {
			var key trafficCursorKey
			v := &trafficCursorValue{}
			if err = rows.Scan(&key.agent, &key.stream, &v.node, &v.sequence, &v.digest, &v.closed, &v.gaps); err != nil {
				rows.Close()
				return err
			}
			b.cursors[key] = v
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
