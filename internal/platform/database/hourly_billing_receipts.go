package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

const hourlySettlementColumns = `id,enrollment_id,hour_start,charged_from,paid_until,hourly_price_minor,charged_minor,offer_revision,policy_version,ledger_id,created_at`

func scanHourlySettlement(row rowScanner) (model.HourlySettlement, error) {
	var r model.HourlySettlement
	err := row.Scan(&r.ID, &r.EnrollmentID, &r.HourStart, &r.ChargedFrom, &r.PaidUntil, &r.HourlyPriceTXBMinor,
		&r.ChargedTXBMinor, &r.OfferRevision, &r.PolicyVersion, &r.LedgerID, &r.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	r.HourStart, r.ChargedFrom, r.PaidUntil, r.CreatedAt = r.HourStart.UTC(), r.ChargedFrom.UTC(), r.PaidUntil.UTC(), r.CreatedAt.UTC()
	r.HourlyPrice, r.Charged = model.TXBMoney(r.HourlyPriceTXBMinor), model.TXBMoney(r.ChargedTXBMinor)
	return r, err
}

func hourlyInitialReceiptTx(ctx context.Context, tx *sql.Tx, enrollment string) (*model.HourlySettlement, error) {
	r, err := scanHourlySettlement(tx.QueryRowContext(ctx, `SELECT `+hourlySettlementColumns+
		` FROM hourly_settlements WHERE enrollment_id=$1 ORDER BY hour_start LIMIT 1`, enrollment))
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	return &r, err
}

// payHourlyIntervalTx charges one deliverable interval whose transaction time is
// also its interval start. Activation and late recovery settle from the current
// instant, so both clocks coincide.
func payHourlyIntervalTx(ctx context.Context, tx *sql.Tx, e model.HourlyEnrollment, offer model.HourlyOffer,
	snapshot model.HourlyPolicySnapshot, from time.Time) (model.HourlySettlement, error) {
	return payHourlyIntervalAtTx(ctx, tx, e, offer, snapshot, from, from)
}

// payHourlyIntervalAtTx separates the charged interval from the actual charge
// instant. A bounded prefetch pays the next whole hour starting at the current
// paid boundary while wallet stamps and the immutable ledger keep transaction
// time, so accounting never records a future-dated debit.
func payHourlyIntervalAtTx(ctx context.Context, tx *sql.Tx, e model.HourlyEnrollment, offer model.HourlyOffer,
	snapshot model.HourlyPolicySnapshot, from, at time.Time) (model.HourlySettlement, error) {
	until := hourlyBoundary(from)
	amount, err := hourlyCharge(offer.HourlyPriceTXBMinor, from, until)
	if err != nil {
		return model.HourlySettlement{}, err
	}
	id, err := ids.New()
	if err != nil {
		return model.HourlySettlement{}, err
	}
	balance, err := debitBalanceTx(ctx, tx, e.OwnerID, amount, at)
	if err != nil {
		return model.HourlySettlement{}, err
	}
	ledger, err := insertLedgerTx(ctx, tx, e.OwnerID, -amount, balance, "hourly_charge", id, e.GroupID, at)
	if err != nil {
		return model.HourlySettlement{}, err
	}
	r := model.HourlySettlement{ID: id, EnrollmentID: e.ID, HourStart: from.UTC().Truncate(time.Hour), ChargedFrom: from,
		PaidUntil: until, HourlyPriceTXBMinor: offer.HourlyPriceTXBMinor, ChargedTXBMinor: amount,
		OfferRevision: offer.Revision, PolicyVersion: snapshot.Version, LedgerID: ledger, CreatedAt: at}
	_, err = tx.ExecContext(ctx, `INSERT INTO hourly_settlements(`+hourlySettlementColumns+`)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, r.ID, r.EnrollmentID, r.HourStart, r.ChargedFrom, r.PaidUntil,
		r.HourlyPriceTXBMinor, r.ChargedTXBMinor, r.OfferRevision, r.PolicyVersion, r.LedgerID, r.CreatedAt)
	r.HourlyPrice, r.Charged = model.TXBMoney(r.HourlyPriceTXBMinor), model.TXBMoney(r.ChargedTXBMinor)
	return r, err
}

func hourlyPaidWindowStillLiveTx(ctx context.Context, tx *sql.Tx, receipts []model.HourlySettlement) error {
	if len(receipts) == 0 {
		return nil
	}
	now, err := hourlyBillingClockTx(ctx, tx)
	if err != nil {
		return err
	}
	for _, receipt := range receipts {
		if !receipt.PaidUntil.After(now) {
			return ErrConflict
		}
	}
	return nil
}
