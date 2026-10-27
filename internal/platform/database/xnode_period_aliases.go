package database

import (
	"context"
	"database/sql"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// resolveTrafficPeriodsTx leaves authenticated report data and digest unchanged.
// An existing period takes precedence over an alias, and a mapped sample must
// match the canonical owner exactly. One indexed read covers the entire report.
func resolveTrafficPeriodsTx(ctx context.Context, tx *sql.Tx, node string, input []wire.TrafficSample) ([]wire.TrafficSample, error) {
	type resolved struct {
		id    string
		owner int64
	}
	aliases := make(map[string]resolved)
	rows, err := tx.QueryContext(ctx, `SELECT a.historical_period_id,p.id,p.proxy_user_id FROM xnode_period_aliases a JOIN xnode_periods p ON p.id=a.canonical_period_id WHERE a.node_id=? AND NOT EXISTS(SELECT 1 FROM xnode_periods existing WHERE existing.id=a.historical_period_id)`, node)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var original string
		var value resolved
		if err = rows.Scan(&original, &value.id, &value.owner); err != nil {
			rows.Close()
			return nil, err
		}
		aliases[original] = value
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	samples := append([]wire.TrafficSample(nil), input...)
	for i := range samples {
		sample := &samples[i]
		if !wire.ValidName(sample.AccountingPeriod) {
			return nil, xnode.ErrInvalid
		}
		if value, ok := aliases[sample.AccountingPeriod]; ok {
			if int64(sample.UserID) != value.owner {
				return nil, xnode.ErrTrafficOwnerMismatch
			}
			sample.AccountingPeriod = value.id
		}
	}
	return samples, nil
}
