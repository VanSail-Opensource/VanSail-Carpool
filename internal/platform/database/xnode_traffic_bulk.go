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

const trafficSQLRows = 512

func nativeKeys[V any](values map[nativeUsageKey]V) []nativeUsageKey {
	keys := slices.Collect(maps.Keys(values))
	slices.SortFunc(keys, func(a, b nativeUsageKey) int {
		return cmp.Or(cmp.Compare(a.period, b.period), cmp.Compare(a.user, b.user), cmp.Compare(a.node, b.node), cmp.Compare(a.date, b.date))
	})
	return keys
}

type nativeUsageColumns struct {
	users                                   []int64
	nodes, periods, dates                   []string
	uploads, downloads, weights, remainders []int64
}

func nativeColumns(keys []nativeUsageKey, values map[nativeUsageKey]nativeUsageValue) nativeUsageColumns {
	var out nativeUsageColumns
	for _, key := range keys {
		out.users = append(out.users, key.user)
		out.nodes, out.periods, out.dates = append(out.nodes, key.node), append(out.periods, key.period), append(out.dates, key.date)
		if values != nil {
			v := values[key]
			out.uploads, out.downloads = append(out.uploads, v.upload), append(out.downloads, v.download)
			out.weights, out.remainders = append(out.weights, v.weighted), append(out.remainders, v.remainder)
		}
	}
	return out
}

func loadNativeTraffic(ctx context.Context, tx *sql.Tx, reports []xnode.TrafficCommit) (*nativeTrafficBuffer, error) {
	b := &nativeTrafficBuffer{periods: map[string]nativePeriodValue{}, daily: map[nativeUsageKey]nativeUsageValue{}, known: map[nativeUsageKey]bool{}, changed: map[nativeUsageKey]nativeUsageValue{}, used: map[string]int64{}}
	periods := map[string]bool{}
	for _, item := range reports {
		for _, sample := range item.Report.Samples {
			periods[sample.AccountingPeriod] = true
			b.known[nativeUsageKey{int64(sample.UserID), item.NodeID, sample.AccountingPeriod, sample.Through.UTC().Format(time.DateOnly)}] = true
		}
	}
	ids := slices.Sorted(maps.Keys(periods))
	for start := 0; start < len(ids); start += trafficSQLRows {
		rows, err := tx.QueryContext(ctx, `SELECT p.id,p.proxy_user_id,p.status,p.used_bytes,EXISTS(SELECT 1 FROM xnode_payg_accounts a WHERE a.period_id=p.id) FROM xnode_periods p WHERE p.id=ANY($1::text[]) ORDER BY p.id FOR UPDATE OF p`, ids[start:min(start+trafficSQLRows, len(ids))])
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			var v nativePeriodValue
			if err = rows.Scan(&id, &v.owner, &v.status, &v.used, &v.payg); err != nil {
				rows.Close()
				return nil, err
			}
			b.periods[id] = v
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	keys := nativeKeys(b.known)
	for start := 0; start < len(keys); start += trafficSQLRows {
		c := nativeColumns(keys[start:min(start+trafficSQLRows, len(keys))], nil)
		rows, err := tx.QueryContext(ctx, `SELECT d.proxy_user_id,d.node_id,d.period_id,d.date,d.upload_bytes,d.download_bytes,d.weighted_bytes,d.weight_remainder FROM xnode_daily_usage d JOIN unnest($1::bigint[],$2::text[],$3::text[],$4::text[]) AS k(user_id,node_id,period_id,date) ON d.proxy_user_id=k.user_id AND d.node_id=k.node_id AND d.period_id=k.period_id AND d.date=k.date`, c.users, c.nodes, c.periods, c.dates)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var key nativeUsageKey
			var v nativeUsageValue
			if err = rows.Scan(&key.user, &key.node, &key.period, &key.date, &v.upload, &v.download, &v.weighted, &v.remainder); err != nil {
				rows.Close()
				return nil, err
			}
			b.daily[key] = v
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return b, nil
}
