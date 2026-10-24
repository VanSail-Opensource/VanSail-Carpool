package database

import (
	"context"
	"database/sql"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// attributeProbeSample keeps the original journal/sequence deduplication and
// multiplier rounding. It changes attribution only with an immutable exact
// probe/period/target reference. Late samples after expiry/revocation still count;
// period closure requires every probe barrier before financial settlement.
func attributeProbeSample(ctx context.Context, tx *sql.Tx, node string, owner int64, sample *wire.TrafficSample) error {
	var allowed bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM xnode_metered_probes WHERE proxy_user_id=? AND period_id=? AND node_id=?)`, sample.UserID, sample.AccountingPeriod, node).Scan(&allowed); err != nil {
		return err
	}
	if !allowed {
		return xnode.ErrTrafficOwnerMismatch
	}
	sample.UserID = int(owner)
	return nil
}
