package database

import (
	"context"
	"database/sql"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/iplookup"
)

// lockIPLookupRunTx fences running report mutations while holding the row lock
// through their commit. Terminal rows can only be replayed from stored evidence.
// A missing context fence never grants access to an already leased report.
func lockIPLookupRunTx(ctx context.Context, tx *sql.Tx, id string) (status, raw string, err error) {
	var owned bool
	err = tx.QueryRowContext(ctx, `SELECT status,report_json,
		COALESCE((COALESCE(lease_owner,'')='' AND $2='') OR
		(lease_owner=$2 AND `+purchaseTimestampOrderSQL("lease_expires_at")+`>`+databaseClockTimestampOrderSQL+`),false)
		FROM ip_lookup_reports WHERE id=$1 FOR UPDATE`, id, iplookup.RunLeaseOwner(ctx)).Scan(&status, &raw, &owned)
	if err == sql.ErrNoRows || (err == nil && status == "processing" && !owned) {
		err = ErrConflict
	}
	return status, raw, err
}

func lockIPLookupProcessingRunTx(ctx context.Context, tx *sql.Tx, id string) error {
	status, _, err := lockIPLookupRunTx(ctx, tx, id)
	if err == nil && status != "processing" {
		return ErrConflict
	}
	return err
}
