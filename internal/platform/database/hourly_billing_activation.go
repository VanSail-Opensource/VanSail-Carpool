package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// CommitHourlyActivation charges only after the native tuple is durably ready.
// It rechecks admission and starts the paid interval after all wallet waits.
func (s *Store) CommitHourlyActivation(ctx context.Context, input HourlyCommitInput,
	snapshot model.HourlyPolicySnapshot, _ time.Time) (HourlyActivationResult, error) {
	fingerprint, err := hourlyActivationFingerprint(input.HourlyActivationInput)
	if err != nil || input.EnrollmentID == "" || input.ProxyUserID <= 0 || input.PeriodID == "" {
		return HourlyActivationResult{}, ErrConflict
	}
	return hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (HourlyActivationResult, error) {
		if err := lockHourlyOwnerTx(ctx, tx, input.OwnerID, input.AccountGeneration); err != nil {
			return HourlyActivationResult{}, err
		}
		balance, err := balanceTx(ctx, tx, input.OwnerID)
		if err != nil {
			return HourlyActivationResult{}, err
		}
		e, err := scanHourlyEnrollment(tx.QueryRowContext(ctx, `SELECT `+hourlyEnrollmentColumns+
			` FROM hourly_enrollments WHERE id=$1 AND owner_id=$2 AND account_generation=$3 FOR UPDATE`,
			input.EnrollmentID, input.OwnerID, input.AccountGeneration))
		if err != nil {
			return HourlyActivationResult{}, err
		}
		if e.Fingerprint != fingerprint || e.ActivationKey != input.ActivationKey || e.GroupID != input.GroupID ||
			e.ProxyUserID != input.ProxyUserID || e.PeriodID != input.PeriodID {
			return HourlyActivationResult{}, ErrConflict
		}
		if e.Status != model.HourlyPreparing {
			receipt, err := hourlyInitialReceiptTx(ctx, tx, e.ID)
			if err == nil && receipt == nil {
				err = ErrConflict
			}
			return HourlyActivationResult{e, receipt}, err
		}
		if err = validateHourlyPreparedTupleTx(ctx, tx, e); err != nil {
			return HourlyActivationResult{}, err
		}
		if err = hourlyActivationGroupTx(ctx, tx, e.GroupID); err != nil {
			return HourlyActivationResult{}, err
		}
		if err = lockSquadStockTx(ctx, tx, []string{e.GroupID}); err != nil {
			return HourlyActivationResult{}, err
		}
		offer, err := hourlyActivationChecksTx(ctx, tx, input.HourlyActivationInput, snapshot, balance)
		if err != nil {
			return HourlyActivationResult{}, err
		}
		if err = checkSquadStockTx(ctx, tx, []string{e.GroupID}, e.OwnerID); err != nil {
			return HourlyActivationResult{}, err
		}
		now, err := hourlyBillingClockTx(ctx, tx)
		if err != nil {
			return HourlyActivationResult{}, err
		}
		if input.Delivery.EnrollmentID != e.ID || !hourlyDeliveryFresh(input.Delivery, now) {
			return HourlyActivationResult{}, ErrHourlyDeliveryPending
		}
		revision, err := hourlyCompileRevisionTx(ctx, tx)
		if err != nil {
			return HourlyActivationResult{}, err
		}
		if input.ExpectedCompileRevision <= 0 || revision != input.ExpectedCompileRevision ||
			input.Delivery.CompileRevision > 0 && input.Delivery.CompileRevision != input.ExpectedCompileRevision {
			return HourlyActivationResult{}, ErrHourlyDeliveryPending
		}
		if err = requireHourlyOwnerCapacityTx(ctx, tx, e.OwnerID, e.AccountGeneration, maxHourlyActiveOwners); err != nil {
			return HourlyActivationResult{}, err
		}
		receipt, err := payHourlyIntervalTx(ctx, tx, e, offer, snapshot, now)
		if err != nil {
			return HourlyActivationResult{}, err
		}
		e, err = scanHourlyEnrollment(tx.QueryRowContext(ctx, `UPDATE hourly_enrollments SET status='active',activated_at=$1,
			paid_through=$2,updated_at=$1 WHERE id=$3 AND status='preparing' RETURNING `+hourlyEnrollmentColumns,
			now, receipt.PaidUntil, e.ID))
		if err != nil {
			return HourlyActivationResult{}, err
		}
		if err = enqueueHourlySyncTx(ctx, tx, e.OwnerID, now); err != nil {
			return HourlyActivationResult{}, err
		}
		if err = hourlyPaidDeliveryStillReadyTx(ctx, tx, []model.HourlySettlement{receipt}, map[string]HourlyDeliveryReady{e.ID: input.Delivery}); err != nil {
			return HourlyActivationResult{}, err
		}
		if err = hourlyPaidWindowStillLiveTx(ctx, tx, []model.HourlySettlement{receipt}); err != nil {
			return HourlyActivationResult{}, err
		}
		return HourlyActivationResult{e, &receipt}, nil
	})
}

func validateHourlyPreparedTupleTx(ctx context.Context, tx *sql.Tx, e model.HourlyEnrollment) error {
	var valid bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM xnode_identities i JOIN xnode_periods p ON p.id=i.current_period_id
		WHERE i.proxy_user_id=$1 AND i.service_name=$2 AND i.credentials_cipher<>'' AND i.access_disabled=0
		AND p.id=$3 AND p.proxy_user_id=i.proxy_user_id AND p.strategy='NO_RESET' AND p.status='open' AND p.purchase_id IS NULL)`,
		e.ProxyUserID, "member-hourly-"+e.ID, e.PeriodID).Scan(&valid)
	if err == nil && (!valid || e.PeriodID != "hourly-"+e.ID) {
		err = ErrConflict
	}
	return err
}
