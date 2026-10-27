package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// Only explicit accounting-reference conflicts are eligible for discard.
// Storage errors, invalid frames and agent authentication remain failures.
func discardedTrafficReason(err error) string {
	for _, item := range []struct {
		err  error
		code string
	}{
		{xnode.ErrTrafficPeriodMissing, "period_missing"},
		{xnode.ErrTrafficPAYGPeriodMissing, "payg_period_missing"},
		{xnode.ErrTrafficIdentityMissing, "identity_missing"},
		{xnode.ErrTrafficOwnerMismatch, "owner_mismatch"},
		{xnode.ErrTrafficSampleMissing, "sample_missing"},
		{xnode.ErrTrafficPeriodSettled, "period_settled"},
	} {
		if errors.Is(err, item.err) {
			return item.code
		}
	}
	return ""
}

// An audit record and the real report cursor are committed atomically. Retried
// frames reuse the same audit ID; no missing sequence or digest is fabricated.
func recordDiscardedTrafficTx(ctx context.Context, tx *sql.Tx, node, agent string, sequence, previous uint64, digest, reason, missingPeriod string, discarded bool, report wire.TrafficReport) error {
	detail := map[string]any{
		"nodeId": node, "agentId": agent, "sequence": strconv.FormatUint(sequence, 10),
		"previousSequence": strconv.FormatUint(previous, 10), "reason": reason,
		"discardedReport": discarded, "sampleCount": len(report.Samples), "aggregateUsers": len(report.Traffic),
	}
	if sequence > previous+1 {
		detail["missingFrom"] = strconv.FormatUint(previous+1, 10)
		detail["missingThrough"] = strconv.FormatUint(sequence-1, 10)
	}
	if missingPeriod != "" && wire.ValidName(missingPeriod) {
		detail["missingPeriod"] = missingPeriod
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	key := sha256.Sum256([]byte(node + "\n" + agent + "\n" + strconv.FormatUint(sequence, 10) + "\n" + digest))
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_events(id,actor_user_id,action,target_type,target_id,detail,created_at) VALUES(?,NULL,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`, "traffic-discard-"+hex.EncodeToString(key[:]), "xnode_traffic_discard", "xnode_node", node, string(raw), stamp(time.Now().UTC()))
	return err
}
