package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// CompareAndSwapHourlyPolicy atomically commits encrypted policy and audit.
// An advisory lock serializes absent-row setup; FOR UPDATE also serializes
// existing-row changes with the commerce transaction's FOR SHARE version fence.
func (s *Store) CompareAndSwapHourlyPolicy(ctx context.Context, actor, expectedVersion, ciphertext string) (HourlyPolicyRecord, error) {
	if actor == "" || ciphertext == "" || len(ciphertext) > maxHourlyPolicyCipherBytes || expectedVersion != "" && !validHourlyCreditHex(expectedVersion, 64) {
		return HourlyPolicyRecord{}, ErrConflict
	}
	return hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (HourlyPolicyRecord, error) {
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "settings:"+HourlyPolicySettingKey); err != nil {
			return HourlyPolicyRecord{}, err
		}
		current, err := hourlyPolicyRecord(ctx, tx, " FOR UPDATE")
		if err != nil {
			return HourlyPolicyRecord{}, err
		}
		if current.Version != expectedVersion {
			return HourlyPolicyRecord{}, ErrConflict
		}
		now := time.Now().UTC()
		_, err = tx.ExecContext(ctx, `INSERT INTO settings(key,value,encrypted,updated_at,updated_by) VALUES($1,$2,1,$3,$4) ON CONFLICT(key) DO UPDATE SET value=excluded.value,encrypted=1,updated_at=excluded.updated_at,updated_by=excluded.updated_by`, HourlyPolicySettingKey, ciphertext, stamp(now), actor)
		if err != nil {
			return HourlyPolicyRecord{}, err
		}
		next := HourlyPolicyRecord{Ciphertext: ciphertext, Version: hourlyPolicyFingerprint(ciphertext)}
		id, err := ids.New()
		if err != nil {
			return HourlyPolicyRecord{}, err
		}
		detail, err := json.Marshal(map[string]any{"version": next.Version, "encrypted": true})
		if err != nil {
			return HourlyPolicyRecord{}, err
		}
		if err = insertAuditTx(ctx, tx, id, &actor, "xnode.hourly_policy.update", "setting", HourlyPolicySettingKey, string(detail), now); err != nil {
			return HourlyPolicyRecord{}, err
		}
		return next, nil
	})
}
