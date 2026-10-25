package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrAutoRenewalEnabled = errors.New("automatic renewal is enabled")

type renewalAdmissionReader interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

// Parse exact instants: RFC3339Nano strings do not have uniform lexical order.
func renewalBlockedUntil(ctx context.Context, reader renewalAdmissionReader, user string, now time.Time) (*time.Time, error) {
	rows, err := reader.QueryContext(ctx, `SELECT valid_from,valid_until FROM purchases
		WHERE user_id=? AND auto_renew_enabled=1 AND status IN ('active','activating')`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var blocked *time.Time
	for rows.Next() {
		var rawFrom, rawUntil string
		if err := rows.Scan(&rawFrom, &rawUntil); err != nil {
			return nil, err
		}
		from, err := parseStamp(rawFrom)
		if err != nil {
			return nil, err
		}
		until, err := parseStamp(rawUntil)
		if err != nil {
			return nil, err
		}
		if !now.Before(from) && now.Before(until) && (blocked == nil || until.After(*blocked)) {
			value := until
			blocked = &value
		}
	}
	return blocked, rows.Err()
}

func (s *Store) AutoRenewalBlockedUntil(ctx context.Context, user string, now time.Time) (*time.Time, error) {
	return renewalBlockedUntil(ctx, s.db, user, now)
}

// One member balance row serializes manual checkout, renewal, and enablement.
// SQLite already holds its writer lock; PostgreSQL uses this existing row lock.
func lockMemberPurchaseTx(ctx context.Context, tx *sql.Tx, user string) error {
	var owner string
	return tx.QueryRowContext(ctx, `SELECT user_id FROM balances WHERE user_id=? /*pg:FOR UPDATE*/`, user).Scan(&owner)
}
