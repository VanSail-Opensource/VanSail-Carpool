package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

const hourlyFundingWarningLimit = 200

// EnqueueHourlyFundingWarnings queues at most one warning per enrolled squad and
// paid-through boundary. It only warns owners whose active enrollment ends
// within the next hour and whose balance cannot fund the next hourly price, so
// a top-up before the boundary keeps the squad running.
func (s *Store) EnqueueHourlyFundingWarnings(ctx context.Context, now time.Time) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT e.id,e.owner_id,e.paid_through,o.hourly_price_minor,
		COALESCE(b.txb_minor,0),COALESCE(NULLIF(g.name,''),e.group_id)
		FROM hourly_enrollments e
		JOIN users u ON u.id=e.owner_id AND u.account_status='active' AND u.account_generation=e.account_generation
		JOIN hourly_offers o ON o.group_id=e.group_id AND o.hourly_price_minor>0
		LEFT JOIN xnode_groups g ON g.id=e.group_id
		LEFT JOIN balances b ON b.user_id=e.owner_id
		WHERE e.status='active' AND e.paid_through>$1 AND e.paid_through<=$2
		AND o.hourly_price_minor>COALESCE(b.txb_minor,0)
		ORDER BY e.paid_through,e.id LIMIT $3`, now.UTC(), now.UTC().Add(time.Hour), hourlyFundingWarningLimit)
	if err != nil {
		return 0, fmt.Errorf("list hourly funding warnings: %w", err)
	}
	type warning struct {
		enrollmentID, ownerID, squad string
		paidThrough                  time.Time
		price, balance               int64
	}
	items := make([]warning, 0, hourlyFundingWarningLimit)
	for rows.Next() {
		var item warning
		if err := rows.Scan(&item.enrollmentID, &item.ownerID, &item.paidThrough,
			&item.price, &item.balance, &item.squad); err != nil {
			_ = rows.Close()
			return 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, err
	}
	_ = rows.Close()
	queued := 0
	for _, item := range items {
		inserted, err := s.insertUserNotificationTx(ctx, tx,
			hourlyFundingEventKey("hourly-funding-warning", item.enrollmentID, item.paidThrough), item.ownerID,
			jobpayload.UserEventHourlyFundingWarning, "",
			hourlyFundingFacts(item.squad, item.price, item.balance, item.paidThrough), now)
		if err != nil {
			return 0, fmt.Errorf("queue hourly funding warning: %w", err)
		}
		if inserted {
			queued++
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return queued, nil
}

// enqueueHourlyFundingSuspendedTx records the definitive stop inside the
// settlement transaction that changed the enrollment to funding_suspended.
func (s *Store) enqueueHourlyFundingSuspendedTx(ctx context.Context, tx *sql.Tx, e model.HourlyEnrollment,
	offer model.HourlyOffer, at time.Time) error {
	if e.PaidThrough == nil {
		return nil
	}
	squad := e.GroupID
	err := tx.QueryRowContext(ctx, `SELECT COALESCE(NULLIF(name,''),id) FROM xnode_groups WHERE id=$1`, e.GroupID).Scan(&squad)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	var balance int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE((SELECT txb_minor FROM balances WHERE user_id=$1),0)`,
		e.OwnerID).Scan(&balance); err != nil {
		return err
	}
	_, err = s.insertUserNotificationTx(ctx, tx,
		hourlyFundingEventKey("hourly-funding-suspended", e.ID, *e.PaidThrough), e.OwnerID,
		jobpayload.UserEventHourlyFundingSuspended, "",
		hourlyFundingFacts(squad, offer.HourlyPriceTXBMinor, balance, at), at)
	return err
}

func hourlyFundingEventKey(prefix, enrollmentID string, paidThrough time.Time) string {
	return prefix + ":" + enrollmentID + ":" + strconv.FormatInt(paidThrough.Unix(), 10)
}

func hourlyFundingFacts(squad string, price, balance int64, at time.Time) map[string]string {
	return map[string]string{
		notifications.FactSquad:       squad,
		notifications.FactHourlyPrice: strconv.FormatInt(price, 10),
		notifications.FactBalance:     strconv.FormatInt(balance, 10),
		notifications.FactTime:        at.UTC().Format(time.RFC3339Nano),
	}
}
