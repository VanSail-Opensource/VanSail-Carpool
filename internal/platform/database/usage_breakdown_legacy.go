package database

import (
	"context"
	"database/sql"
	"errors"
	"math/big"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type usageLegacy struct {
	id, status             string
	numerator, denominator *big.Int
}

func usageBreakdownLegacyTx(ctx context.Context, tx *sql.Tx, scope usageBreakdownScope) (usageLegacy, error) {
	legacy := usageLegacy{status: model.UsageLegacyNone, numerator: new(big.Int), denominator: big.NewInt(1)}
	var price int64
	var seconds sql.NullInt64
	// charged_txb_minor already includes all paid addon adjustments. Joining
	// addon rows again would double count; catalog validity is intentionally absent.
	err := tx.QueryRowContext(ctx, `SELECT id,charged_txb_minor,purchased_term_seconds FROM purchases WHERE user_id=$1
 AND status IN('active','activating') AND `+purchaseTimestampOrderSQL("valid_from")+`<=$2 AND `+purchaseTimestampOrderSQL("valid_until")+`>$2
 AND `+purchaseTimestampOrderSQL("created_at")+`>=$3 ORDER BY `+purchaseTimestampOrderSQL("valid_from")+` DESC,`+purchaseTimestampOrderSQL("created_at")+` DESC,id LIMIT 1`, scope.owner, purchaseTimestampOrder(scope.to), purchaseTimestampOrder(scope.lifecycle)).Scan(&legacy.id, &price, &seconds)
	if errors.Is(err, sql.ErrNoRows) {
		return legacy, nil
	}
	if err != nil {
		return legacy, err
	}
	if price < 0 {
		return legacy, ErrConflict
	}
	if !seconds.Valid {
		legacy.status = model.UsageLegacyOriginalTermUnknown
		return legacy, nil
	}
	if seconds.Int64 <= 0 {
		return legacy, ErrConflict
	}
	legacy.status = model.UsageLegacyAvailable
	legacy.numerator.Mul(big.NewInt(price), big.NewInt(3*24*60*60))
	legacy.denominator.SetInt64(seconds.Int64)
	return legacy, nil
}
