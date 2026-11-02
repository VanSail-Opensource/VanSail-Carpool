package database

import (
	"context"
	"database/sql"
	"time"
)

type raffleTicket struct {
	ID, UserID   string
	Fee, Reserve int64
}

func activeRaffleTicketsTx(ctx context.Context, tx *sql.Tx, drawID string) ([]raffleTicket, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id,user_id,fee_minor,reserve_minor FROM activity_raffle_tickets
  WHERE draw_id=$1 AND status='active' ORDER BY created_at,id`, drawID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	tickets := make([]raffleTicket, 0)
	for rows.Next() {
		var ticket raffleTicket
		if err = rows.Scan(&ticket.ID, &ticket.UserID, &ticket.Fee, &ticket.Reserve); err != nil {
			return nil, err
		}
		tickets = append(tickets, ticket)
	}
	return tickets, rows.Err()
}

func refundRaffleTicketTx(ctx context.Context, tx *sql.Tx, ticket raffleTicket, drawName string, now time.Time) error {
	refund := ticket.Fee + ticket.Reserve
	balance, err := changeBalanceTx(ctx, tx, ticket.UserID, refund, now)
	if err != nil {
		return err
	}
	if _, err = insertLedgerTx(ctx, tx, ticket.UserID, refund, balance, "activity_raffle_refund", ticket.ID, drawName, now); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE activity_raffle_tickets SET status='refunded',updated_at=$1 WHERE id=$2 AND status='active'`, stamp(now), ticket.ID)
	return err
}
