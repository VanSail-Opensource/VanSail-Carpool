package database

import (
	"context"
	"database/sql"
	"errors"
	"math"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

type nativeUsageKey struct {
	user               int64
	node, period, date string
}
type nativeUsageValue struct{ upload, download, weighted, remainder int64 }
type nativePeriodValue struct {
	owner  int64
	status string
	used   int64
	payg   bool
}

// These maps live only inside one serializable transaction. Financial/admission
// state is never cached between commits. Each report stages independently so a
// savepoint rollback cannot leave partial counters in Go memory.
type nativeTrafficBuffer struct {
	periods map[string]nativePeriodValue
	daily   map[nativeUsageKey]nativeUsageValue
	known   map[nativeUsageKey]bool
	changed map[nativeUsageKey]nativeUsageValue
	used    map[string]int64
}
type nativeReportBuffer struct {
	parent *nativeTrafficBuffer
	daily  map[nativeUsageKey]nativeUsageValue
	used   map[string]int64
}

func (b *nativeTrafficBuffer) fork() *nativeReportBuffer {
	return &nativeReportBuffer{parent: b, daily: map[nativeUsageKey]nativeUsageValue{}, used: map[string]int64{}}
}
func (b *nativeReportBuffer) accept() {
	for key, value := range b.daily {
		b.parent.changed[key] = value
	}
	for period, bytes := range b.used {
		b.parent.used[period] += bytes
	}
}

func nativePeriodTx(ctx context.Context, tx *sql.Tx, b *nativeReportBuffer, id string) (nativePeriodValue, error) {
	if b != nil {
		value, exists := b.parent.periods[id]
		if !exists {
			return value, xnode.NewTrafficPeriodMissing(id)
		}
		return value, nil
	}
	var value nativePeriodValue
	err := tx.QueryRowContext(ctx, `SELECT proxy_user_id,status,used_bytes FROM xnode_periods WHERE id=$1`, id).Scan(&value.owner, &value.status, &value.used)
	if errors.Is(err, sql.ErrNoRows) {
		err = xnode.NewTrafficPeriodMissing(id)
	}
	return value, err
}

func nativeUsageTx(ctx context.Context, tx *sql.Tx, b *nativeReportBuffer, key nativeUsageKey) (nativeUsageValue, error) {
	if b != nil {
		if value, exists := b.daily[key]; exists {
			return value, nil
		}
		if value, exists := b.parent.changed[key]; exists {
			return value, nil
		}
		if b.parent.known[key] {
			return b.parent.daily[key], nil
		}
	}
	var value nativeUsageValue
	err := tx.QueryRowContext(ctx, `SELECT upload_bytes,download_bytes,weighted_bytes,weight_remainder FROM xnode_daily_usage WHERE proxy_user_id=$1 AND node_id=$2 AND period_id=$3 AND date=$4`, key.user, key.node, key.period, key.date).Scan(&value.upload, &value.download, &value.weighted, &value.remainder)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	if err == nil && b != nil {
		b.parent.daily[key], b.parent.known[key] = value, true
	}
	return value, err
}

func (b *nativeReportBuffer) stage(key nativeUsageKey, value nativeUsageValue, weighted int64) error {
	base := b.parent.periods[key.period].used
	prior := b.parent.used[key.period]
	local := b.used[key.period]
	if base > math.MaxInt64-prior || base+prior > math.MaxInt64-local || base+prior+local > math.MaxInt64-weighted {
		return xnode.ErrInvalid
	}
	b.daily[key] = value
	if weighted > 0 {
		b.used[key.period] = local + weighted
	}
	return nil
}
