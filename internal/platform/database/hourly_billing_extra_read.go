package database

import (
	"context"
	"database/sql"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

const hourlyExtraPurchaseColumns = `id,owner_id,account_generation,day_utc::text,block_index,byte_count,unit_price_minor,policy_version,ledger_id,created_at`

func scanHourlyExtraPurchase(row rowScanner) (model.HourlyExtraPurchase, error) {
	var p model.HourlyExtraPurchase
	err := row.Scan(&p.ID, &p.OwnerID, &p.AccountGeneration, &p.DayUTC, &p.BlockIndex, &p.ByteCount,
		&p.UnitPriceTXBMinor, &p.PolicyVersion, &p.LedgerID, &p.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	p.CreatedAt, p.UnitPrice = p.CreatedAt.UTC(), model.TXBMoney(p.UnitPriceTXBMinor)
	return p, err
}

// HourlyExtraPurchase reads one immutable funding receipt for reconciliation.
func (s *Store) HourlyExtraPurchase(ctx context.Context, id string) (model.HourlyExtraPurchase, error) {
	return scanHourlyExtraPurchase(s.db.QueryRowContext(ctx, `SELECT `+hourlyExtraPurchaseColumns+
		` FROM hourly_extra_purchases WHERE id=$1`, id))
}

func hourlyExtraPurchaseTx(ctx context.Context, tx *sql.Tx, id string) (model.HourlyExtraPurchase, error) {
	return scanHourlyExtraPurchase(tx.QueryRowContext(ctx, `SELECT `+hourlyExtraPurchaseColumns+
		` FROM hourly_extra_purchases WHERE id=$1`, id))
}
