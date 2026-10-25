package database

import (
	"context"
	"database/sql"
	"fmt"
)

// Eligibility is a property of the final deletion set, not just status and age.
// Retaining one native/unfinished term also retains every referenced predecessor.
func protectPurchaseLineageTx(ctx context.Context, tx *sql.Tx) error {
	for {
		removed, err := deleteCount(ctx, tx, `DELETE FROM maintenance_purchase_candidates AS candidate
			WHERE EXISTS (SELECT 1 FROM purchases successor
				WHERE successor.auto_renew_source_purchase_id=candidate.id AND NOT EXISTS
				(SELECT 1 FROM maintenance_purchase_candidates eligible WHERE eligible.id=successor.id))
			OR EXISTS (SELECT 1 FROM renewal_batches batch JOIN purchases term ON term.renewal_batch_id=batch.id
				WHERE batch.source_purchase_id=candidate.id AND NOT EXISTS
				(SELECT 1 FROM maintenance_purchase_candidates eligible WHERE eligible.id=term.id))`)
		if err != nil {
			return fmt.Errorf("protect retained purchase lineage: %w", err)
		}
		if removed == 0 {
			return nil
		}
	}
}
