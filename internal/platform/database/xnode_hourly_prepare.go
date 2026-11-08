package database

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// Preparation shares the users-row lifecycle fence with activation and purge.
// Credentials are encrypted before entry; no cryptography or network waits occur
// while locks are held, and concurrent preparation never rotates a live tuple.
func (s *Store) EnsureXNodeHourlyPrincipal(ctx context.Context, enrollment, cipher string) (xnode.HourlyPrincipal, error) {
	if enrollment == "" || cipher == "" {
		return xnode.HourlyPrincipal{}, xnode.ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return xnode.HourlyPrincipal{}, err
	}
	defer tx.Rollback()
	var owner string
	if err = tx.QueryRowContext(ctx, `SELECT owner_id FROM hourly_enrollments WHERE id=$1`, enrollment).Scan(&owner); err != nil {
		return xnode.HourlyPrincipal{}, err
	}
	var generation int64
	var active bool
	if err = tx.QueryRowContext(ctx, `SELECT account_generation,account_status='active' FROM users WHERE id=$1 FOR UPDATE`, owner).Scan(&generation, &active); err != nil {
		return xnode.HourlyPrincipal{}, err
	}
	p, err := scanHourlyPrincipal(tx.QueryRowContext(ctx, `SELECT `+hourlyPrincipalColumns+` FROM hourly_enrollments e WHERE e.id=$1 FOR UPDATE`, enrollment))
	if err != nil {
		return p, err
	}
	if !active || p.OwnerID != owner || generation != p.AccountGeneration {
		return p, ErrAccountUnavailable
	}
	if p.ProxyUserID != 0 || p.PeriodID != "" {
		if p.ProxyUserID <= 0 || p.PeriodID != "hourly-"+p.EnrollmentID {
			return p, xnode.ErrConflict
		}
		var valid bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM xnode_identities i JOIN xnode_periods p ON p.id=i.current_period_id AND p.proxy_user_id=i.proxy_user_id WHERE i.proxy_user_id=$1 AND i.service_name=$2 AND i.credentials_cipher<>'' AND p.id=$3 AND p.strategy='NO_RESET' AND p.status='open' AND p.purchase_id IS NULL)`, p.ProxyUserID, "member-hourly-"+p.EnrollmentID, p.PeriodID).Scan(&valid); err != nil {
			return p, err
		}
		if !valid {
			return p, xnode.ErrConflict
		}
		return p, tx.Commit()
	}
	if p.Status != "preparing" {
		return p, xnode.ErrConflict
	}
	if err = tx.QueryRowContext(ctx, `SELECT nextval('xnode_service_proxy_id_seq')`).Scan(&p.ProxyUserID); err != nil {
		return p, err
	}
	if p.ProxyUserID <= 0 || p.ProxyUserID >= 1<<53 {
		return p, xnode.ErrUnavailable
	}
	now := stamp(time.Now().UTC())
	p.PeriodID = "hourly-" + p.EnrollmentID
	if _, err = tx.ExecContext(ctx, `INSERT INTO xnode_identities(proxy_user_id,service_name,credentials_cipher,subscription_cipher,current_period_id,created_at) VALUES($1,$2,$3,'',$4,$5)`, p.ProxyUserID, "member-hourly-"+p.EnrollmentID, cipher, p.PeriodID, now); err != nil {
		return p, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO xnode_periods(id,proxy_user_id,started_at,limit_bytes,strategy,status) VALUES($1,$2,$3,0,'NO_RESET','open')`, p.PeriodID, p.ProxyUserID, now); err != nil {
		return p, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE hourly_enrollments SET proxy_user_id=$1,period_id=$2,updated_at=$3 WHERE id=$4 AND status='preparing' AND proxy_user_id IS NULL AND period_id IS NULL`, p.ProxyUserID, p.PeriodID, now, enrollment)
	if err != nil {
		return p, err
	}
	if n, e := result.RowsAffected(); e != nil || n != 1 {
		return p, xnode.ErrConflict
	}
	return p, tx.Commit()
}
