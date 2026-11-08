package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// pricingCostEvidenceTx binds public optimization to the exact cost metadata versions.
func pricingCostEvidenceTx(ctx context.Context, tx *sql.Tx, nodes []string, now time.Time) (string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT node_id,version FROM pricing_cost_metadata WHERE effective_at<=$1 ORDER BY node_id LIMIT 4097`, pricingStamp(now))
	if err != nil {
		return "", err
	}
	defer rows.Close()
	versions := map[string]int64{}
	for rows.Next() {
		var id string
		var version int64
		if err = rows.Scan(&id, &version); err != nil {
			return "", err
		}
		if len(versions) >= 4096 {
			return "", ErrConflict
		}
		versions[id] = version
	}
	if err = rows.Err(); err != nil {
		return "", err
	}
	var out strings.Builder
	for _, id := range nodes {
		fmt.Fprintf(&out, "%s:%d;", id, versions[id])
	}
	return out.String(), nil
}
