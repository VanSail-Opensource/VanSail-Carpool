package database

import (
	"context"
	"database/sql"
	"math/big"
	"slices"
	"time"
)

type usageMinuteKey struct {
	owner, mode, reference, period, node, minute string
	principal                                    int64
}
type usageMinuteValue struct {
	upload, download big.Int
	generation       int64 // -1 means an unrecorded legacy period generation.
	from, through    time.Time
}
type usageMinutes map[usageMinuteKey]*usageMinuteValue

func (m usageMinutes) add(key usageMinuteKey, value *usageMinuteValue) {
	previous := m[key]
	if previous == nil {
		previous = &usageMinuteValue{generation: value.generation, from: value.from, through: value.through}
		m[key] = previous
	}
	previous.upload.Add(&previous.upload, &value.upload)
	previous.download.Add(&previous.download, &value.download)
	if value.from.Before(previous.from) {
		previous.from = value.from
	}
	if value.through.After(previous.through) {
		previous.through = value.through
	}
}

func (m usageMinutes) flush(ctx context.Context, tx *sql.Tx) error {
	keys := make([]usageMinuteKey, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(a, b usageMinuteKey) int {
		return slices.Compare([]string{a.owner, a.mode, a.reference, a.period, a.node, a.minute}, []string{b.owner, b.mode, b.reference, b.period, b.node, b.minute})
	})
	for start := 0; start < len(keys); start += trafficSQLRows {
		var owners, modes, references, periods, nodes, minutes, ups, downs []string
		var principals, generations []int64
		var from, through []time.Time
		for _, key := range keys[start:min(start+trafficSQLRows, len(keys))] {
			value := m[key]
			owners, modes, references = append(owners, key.owner), append(modes, key.mode), append(references, key.reference)
			periods, nodes, minutes = append(periods, key.period), append(nodes, key.node), append(minutes, key.minute)
			principals, generations = append(principals, key.principal), append(generations, value.generation)
			from, through = append(from, value.from), append(through, value.through)
			ups, downs = append(ups, value.upload.String()), append(downs, value.download.String())
		}
		result, err := tx.ExecContext(ctx, `INSERT INTO xnode_usage_minutes AS h(owner_id,billing_mode,reference_id,period_id,node_id,minute,proxy_user_id,account_generation,observed_from,observed_through,upload_bytes,download_bytes) SELECT owner,mode,reference,period,node,minute,principal,NULLIF(generation,-1),observed_from,observed_through,upload::numeric,download::numeric FROM unnest($1::text[],$2::text[],$3::text[],$4::text[],$5::text[],$6::timestamptz[],$7::bigint[],$8::bigint[],$9::timestamptz[],$10::timestamptz[],$11::text[],$12::text[]) AS v(owner,mode,reference,period,node,minute,principal,generation,observed_from,observed_through,upload,download) ON CONFLICT(owner_id,billing_mode,reference_id,proxy_user_id,period_id,node_id,minute) DO UPDATE SET upload_bytes=h.upload_bytes+excluded.upload_bytes,download_bytes=h.download_bytes+excluded.download_bytes,observed_from=LEAST(h.observed_from,excluded.observed_from),observed_through=GREATEST(h.observed_through,excluded.observed_through) WHERE h.account_generation IS NOT DISTINCT FROM excluded.account_generation`, owners, modes, references, periods, nodes, minutes, principals, generations, from, through, ups, downs)
		if err != nil {
			return err
		}
		if n, err := result.RowsAffected(); err != nil || n != int64(min(trafficSQLRows, len(keys)-start)) {
			return ErrConflict
		}
	}
	return nil
}

func stageUsageMinute(ctx context.Context, tx *sql.Tx, buffer *nativeReportBuffer, key usageMinuteKey, value *usageMinuteValue) error {
	if buffer != nil {
		if buffer.usageHistory == nil {
			buffer.usageHistory = usageMinutes{}
		}
		buffer.usageHistory.add(key, value)
		return nil
	}
	values := usageMinutes{}
	values.add(key, value)
	return values.flush(ctx, tx)
}
