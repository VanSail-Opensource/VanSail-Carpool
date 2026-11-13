package database

import (
	"context"
	"database/sql"
	"strconv"
	"time"
)

func reconcileRaffleReservationsTx(ctx context.Context, tx *sql.Tx, drawID string, required int64, name string, revision int, now time.Time) error {
	tickets, err := activeRaffleTicketsTx(ctx, tx, drawID)
	if err != nil {
		return err
	}
	for _, ticket := range tickets {
		delta := required - ticket.Reserve
		if delta == 0 {
			continue
		}
		if delta > 0 {
			balance, balanceErr := balanceTx(ctx, tx, ticket.UserID)
			if balanceErr != nil {
				return balanceErr
			}
			if balance < delta {
				return ErrInsufficientBalance
			}
		}
		balance, changeErr := changeBalanceTx(ctx, tx, ticket.UserID, -delta, now)
		if changeErr != nil {
			return changeErr
		}
		if _, err = insertLedgerTx(ctx, tx, ticket.UserID, -delta, balance, "activity_raffle_reserve_adjust", ticket.ID+":"+strconv.Itoa(revision), name, now); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE activity_raffle_tickets SET reserve_minor=$1,updated_at=$2 WHERE id=$3`, required, stamp(now), ticket.ID); err != nil {
			return err
		}
	}
	return nil
}
