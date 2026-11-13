package database

import (
	"context"
	"database/sql"
	"math/big"
)

func usageBreakdownUnallocatedTx(ctx context.Context, tx *sql.Tx, scope usageBreakdownScope) (*big.Int, error) {
	var raw string
	err := tx.QueryRowContext(ctx, `SELECT COALESCE((SELECT SUM(p.unit_price_minor::numeric) FROM hourly_extra_purchases p
 JOIN ledger_entries l ON l.id=p.ledger_id WHERE p.owner_id=$1 AND p.account_generation=$2
 AND l.user_id=$1 AND l.kind='hourly_extra_gb' AND `+purchaseTimestampOrderSQL("l.created_at")+`>=$3
 AND `+purchaseTimestampOrderSQL("l.created_at")+`<=$4),0)::text`, scope.owner, scope.generation, purchaseTimestampOrder(scope.from), purchaseTimestampOrder(scope.to)).Scan(&raw)
	if err != nil {
		return nil, err
	}
	return usageUnsigned(raw)
}
