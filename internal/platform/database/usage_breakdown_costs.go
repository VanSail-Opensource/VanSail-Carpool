package database

import (
	"context"
	"database/sql"
	"math/big"
)

func usageBreakdownUnallocatedTx(ctx context.Context, tx *sql.Tx, scope usageBreakdownScope) (*big.Int, error) {
	var raw string
	err := tx.QueryRowContext(ctx, `SELECT (
 COALESCE((SELECT SUM(s.charged_minor::numeric) FROM hourly_settlements s
 JOIN hourly_enrollments e ON e.id=s.enrollment_id JOIN ledger_entries l ON l.id=s.ledger_id
 WHERE e.owner_id=$1 AND e.account_generation=$2 AND l.user_id=$1 AND l.kind='hourly_charge'
 AND l.created_at::timestamptz>=$3 AND l.created_at::timestamptz<=$4),0)
 +COALESCE((SELECT SUM(p.unit_price_minor::numeric) FROM hourly_extra_purchases p
 JOIN ledger_entries l ON l.id=p.ledger_id WHERE p.owner_id=$1 AND p.account_generation=$2
 AND l.user_id=$1 AND l.kind='hourly_extra_gb' AND l.created_at::timestamptz>=$3
 AND l.created_at::timestamptz<=$4),0))::text`, scope.owner, scope.generation, scope.from, scope.to).Scan(&raw)
	if err != nil {
		return nil, err
	}
	return usageUnsigned(raw)
}
