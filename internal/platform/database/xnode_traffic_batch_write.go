package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (b *trafficBatchState) flushCursors(ctx context.Context, tx *sql.Tx) error {
	for _, streams := range []bool{false, true} {
		var agents, nodes, boots, digests []string
		var sequences []int64
		var gaps []bool
		for _, key := range sortedTrafficCursors(b.cursors) {
			v := b.cursors[key]
			if !v.dirty || (key.stream != "") != streams {
				continue
			}
			agents, nodes, boots = append(agents, key.agent), append(nodes, v.node), append(boots, key.stream)
			digests, sequences, gaps = append(digests, v.digest), append(sequences, int64(v.sequence)), append(gaps, v.gaps)
		}
		if len(agents) == 0 {
			continue
		}
		var result sql.Result
		var err error
		if !streams {
			result, err = tx.ExecContext(ctx, `INSERT INTO xnode_report_cursors(agent_id,node_id,last_sequence,last_digest,updated_at) SELECT a.agent_id,a.node_id,a.seq_no,a.digest,$5 FROM unnest($1::text[],$2::text[],$3::bigint[],$4::text[]) AS a(agent_id,node_id,seq_no,digest) ON CONFLICT(agent_id) DO UPDATE SET last_sequence=excluded.last_sequence,last_digest=excluded.last_digest,updated_at=excluded.updated_at`, agents, nodes, sequences, digests, stamp(time.Now().UTC()))
		} else {
			result, err = tx.ExecContext(ctx, `UPDATE xnode_traffic_streams s SET last_sequence=a.seq_no,last_digest=a.digest,has_gaps=s.has_gaps OR a.gaps,updated_at=$6 FROM unnest($1::text[],$2::text[],$3::bigint[],$4::text[],$5::boolean[]) AS a(agent_id,stream_id,seq_no,digest,gaps) WHERE s.agent_id=a.agent_id AND s.stream_id=a.stream_id`, agents, boots, sequences, digests, gaps, stamp(time.Now().UTC()))
		}
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if count != int64(len(agents)) {
			return xnode.ErrConflict
		}
	}
	return nil
}
