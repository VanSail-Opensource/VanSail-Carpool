package database

import (
	"context"
	"time"
)

// The indexed text bound is only a candidate filter. RFC3339Nano's omitted
// fractional suffix reverses lexical order at each whole-second boundary.
func otherRenewalPurchaseEndsAfter(ctx context.Context, reader renewalAdmissionReader, userID, excludedID string, after time.Time) (bool, error) {
	lower, _ := purchaseWindowCandidateBounds(after)
	rows, err := reader.QueryContext(ctx, `SELECT valid_until FROM purchases WHERE user_id=$1 AND id<>$2
		AND status IN ('active','activating','queued') AND valid_until>=$3`, userID, excludedID, lower)
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var rawUntil string
		if err := rows.Scan(&rawUntil); err != nil {
			return false, err
		}
		until, err := parseStamp(rawUntil)
		if err != nil {
			return false, err
		}
		if until.After(after) {
			return true, nil
		}
	}
	return false, rows.Err()
}
