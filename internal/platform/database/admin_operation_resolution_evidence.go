package database

import (
	"context"
	"database/sql"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// Only these commands commit their intended local changes before exact-state
// user synchronization. Native commands and payment/member settlement flows
// have additional domain dependencies that a generic receipt cannot reconcile.
func adminResolutionEvidenceTx(ctx context.Context, tx *sql.Tx, kind, operationID, resolution string) error {
	switch kind {
	case providerops.KindAdminEntitlementEdit, providerops.KindAdminEntitlementRefund,
		providerops.KindAdminComboReplacement, providerops.KindAdminBulkExtension, providerops.KindLegacyOutageSync:
	default:
		return ErrOperationReconciliationRequired
	}
	rows, err := tx.QueryContext(ctx, `SELECT status,target_type,target_id,attempt_started_at,completed_at
		FROM provider_operation_items WHERE operation_id=$1 ORDER BY item_key FOR SHARE`, operationID)
	if err != nil {
		return err
	}
	defer rows.Close()
	count := 0
	outcomes := map[string]bool{}
	for rows.Next() {
		var status, targetType, targetID string
		var attempted, completed sql.NullString
		if err := rows.Scan(&status, &targetType, &targetID, &attempted, &completed); err != nil {
			return err
		}
		if targetType != "user" || targetID == "" || !attempted.Valid || !completed.Valid {
			return ErrOperationReconciliationRequired
		}
		switch providerops.Status(status) {
		case providerops.StatusSucceeded, providerops.StatusFailed, providerops.StatusCompensated:
		default:
			return ErrOperationReconciliationRequired
		}
		count++
		outcomes[status] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count == 0 || len(outcomes) == 1 && !outcomes[resolution] || len(outcomes) > 1 && resolution != string(providerops.StatusPartial) {
		return ErrOperationReconciliationRequired
	}
	return nil
}
