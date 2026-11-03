package database

import (
	"context"
	"database/sql"
	"maps"
	"math"
	"slices"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (b *nativeTrafficBuffer) flush(ctx context.Context, tx *sql.Tx) error {
	keys := nativeKeys(b.changed)
	for start := 0; start < len(keys); start += trafficSQLRows {
		c := nativeColumns(keys[start:min(start+trafficSQLRows, len(keys))], b.changed)
		if _, err := tx.ExecContext(ctx, `INSERT INTO xnode_daily_usage(proxy_user_id,node_id,period_id,date,upload_bytes,download_bytes,weighted_bytes,weight_remainder) SELECT * FROM unnest($1::bigint[],$2::text[],$3::text[],$4::text[],$5::bigint[],$6::bigint[],$7::bigint[],$8::bigint[]) ON CONFLICT(proxy_user_id,node_id,period_id,date) DO UPDATE SET upload_bytes=excluded.upload_bytes,download_bytes=excluded.download_bytes,weighted_bytes=excluded.weighted_bytes,weight_remainder=excluded.weight_remainder`, c.users, c.nodes, c.periods, c.dates, c.uploads, c.downloads, c.weights, c.remainders); err != nil {
			return err
		}
	}
	periods := slices.Sorted(maps.Keys(b.used))
	for start := 0; start < len(periods); start += trafficSQLRows {
		ids := periods[start:min(start+trafficSQLRows, len(periods))]
		bytes := make([]int64, 0, len(ids))
		for _, id := range ids {
			bytes = append(bytes, b.used[id])
		}
		result, err := tx.ExecContext(ctx, `UPDATE xnode_periods p SET used_bytes=p.used_bytes+v.bytes FROM unnest($1::text[],$2::bigint[]) AS v(id,bytes) WHERE p.id=v.id AND p.used_bytes<=$3-v.bytes`, ids, bytes, int64(math.MaxInt64))
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if count != int64(len(ids)) {
			return xnode.ErrInvalid
		}
	}
	return nil
}
