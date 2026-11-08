package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (s *Store) accountNativePurgeGuardTx(ctx context.Context, tx *sql.Tx, target model.AccountPurge, jobID string) (string, bool, error) {
	if target.UserID == "" || target.Generation < 1 || target.ThroughGeneration < 0 || target.ThroughGeneration >= target.Generation || jobID == "" {
		return "", false, xnode.ErrInvalid
	}
	// Cleanup lock order is health -> users -> evidence. Never acquire cleanup
	// health after locking the user, including on cursor-only progress writes.
	allowed, err := s.accountCleanupAdmissionTx(ctx, tx, target.UserID, target.Generation, time.Now().UTC())
	if err != nil || !allowed {
		return "", false, err
	}
	if err = fenceAccountPurgeOperationsTx(ctx, tx, target.UserID, time.Now().UTC()); err != nil {
		return "", false, err
	}
	var payload string
	err = tx.QueryRowContext(ctx, `SELECT payload FROM outbox_jobs WHERE id=$1 AND kind='account_purge'
 AND status='processing' AND payload::jsonb->>'userId'=$2 AND payload::jsonb->>'generation'=$3 FOR UPDATE`, jobID, target.UserID, strconv.FormatInt(target.Generation, 10)).Scan(&payload)
	if err != nil {
		return "", false, err
	}
	if len(payload) > 65536 {
		return "", false, xnode.ErrInvalid
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal([]byte(payload), &fields) != nil {
		return "", false, xnode.ErrInvalid
	}
	var cursor string
	if raw, exists := fields["nativePeriodCursor"]; exists && (json.Unmarshal(raw, &cursor) != nil || len(cursor) > 512) {
		return "", false, xnode.ErrInvalid
	}
	return cursor, true, nil
}
