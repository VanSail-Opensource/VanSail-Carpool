package database

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// Both single/batch report writers stage raw history with their existing cursor
// transaction. Storage failures refuse commit/ACK; forward coverage is explicit.
func (*Store) HourlyTrafficHistoryReady() bool { return true }

func startUsageHistoryTx(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO xnode_usage_history_state(singleton,started_at) VALUES(true,$1) ON CONFLICT(singleton) DO NOTHING`, time.Now().UTC())
	return err
}

func recordUsageCoverageTx(ctx context.Context, tx *sql.Tx, node, agent string, sequence uint64, digest, reason string, report wire.TrafficReport) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO xnode_usage_coverage(node_id,agent_id,stream_id,sequence,digest,reason,observed_at,dropped_native,dropped_hourly) VALUES($1,$2,$3,$4,$5,$6,$7,$8::numeric,$9::numeric) ON CONFLICT DO NOTHING`, node, agent, report.StreamID, sequence, digest, reason, time.Now().UTC(), strconv.FormatUint(report.DroppedSamples, 10), strconv.FormatUint(report.DroppedHourlySamples, 10))
	return err
}
