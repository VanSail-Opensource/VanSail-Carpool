package database

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

const accountNativePeriodScope = `EXISTS(SELECT 1 FROM xnode_identities i WHERE i.proxy_user_id=p.proxy_user_id AND
 (i.user_id=$1 OR EXISTS(SELECT 1 FROM xnode_payg_accounts a WHERE a.proxy_user_id=i.proxy_user_id AND a.user_id=$1)
 OR EXISTS(SELECT 1 FROM hourly_enrollments e WHERE e.owner_id=$1 AND e.account_generation<=$2
 AND e.proxy_user_id=i.proxy_user_id AND e.period_id=p.id)))`

// AccountNativePurgePage uses the existing generation's outbox payload as its
// sole cursor. Capturing a page does not skip attempts within that page.
func (s *Store) AccountNativePurgePage(ctx context.Context, target model.AccountPurge, jobID string, mark func([]xnode.PurgeActivityScope) error) ([]xnode.Period, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	cursor, allowed, err := s.accountNativePurgeGuardTx(ctx, tx, target, jobID)
	if err != nil || !allowed {
		return nil, false, err
	}
	periods, err := accountNativePurgePeriodsTx(ctx, tx, target, cursor)
	if err == nil && len(periods) == 0 && cursor != "" {
		periods, err = accountNativePurgePeriodsTx(ctx, tx, target, "")
	}
	if err != nil {
		return nil, false, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE hourly_enrollments SET status='closed',updated_at=clock_timestamp()
 WHERE owner_id=$1 AND account_generation<=$2 AND status<>'closed'`, target.UserID, target.ThroughGeneration); err != nil {
		return nil, false, err
	}
	if mark != nil {
		scopes, e := accountPurgeActivityScopesTx(ctx, tx, target)
		if e != nil {
			return nil, false, e
		}
		if err = mark(scopes); err != nil {
			return nil, false, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, false, err
	}
	return periods, true, nil
}

func accountNativePurgePeriodsTx(ctx context.Context, tx *sql.Tx, target model.AccountPurge, cursor string) ([]xnode.Period, error) {
	rows, err := tx.QueryContext(ctx, `SELECT `+xnodePeriodColumns+` FROM xnode_periods p
 WHERE p.status<>'closed' AND `+accountNativePeriodScope+` AND p.id>$3 ORDER BY p.id LIMIT 128`, target.UserID, target.ThroughGeneration, cursor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	periods := make([]xnode.Period, 0, 128)
	for rows.Next() {
		p, e := scanXNodePeriod(rows)
		if e != nil {
			return nil, e
		}
		periods = append(periods, p)
	}
	return periods, rows.Err()
}

// AdvanceAccountNativePurge durably advances before any RPC and fences native
// admission in the same guarded transaction. A crash can repeat, never skip,
// eventual cyclic work; finalized accounting remains independently immutable.
func (s *Store) AdvanceAccountNativePurge(ctx context.Context, target model.AccountPurge, jobID string, expected xnode.Period) (xnode.Period, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return xnode.Period{}, false, err
	}
	defer tx.Rollback()
	_, allowed, err := s.accountNativePurgeGuardTx(ctx, tx, target, jobID)
	if err != nil || !allowed {
		return xnode.Period{}, false, err
	}
	current, err := scanXNodePeriod(tx.QueryRowContext(ctx, `SELECT `+xnodePeriodColumns+` FROM xnode_periods p
 WHERE p.id=$3 AND `+accountNativePeriodScope+` FOR UPDATE`, target.UserID, target.ThroughGeneration, expected.ID))
	if err != nil {
		return xnode.Period{}, false, err
	}
	if current.ProxyUserID != expected.ProxyUserID || current.PurchaseID != expected.PurchaseID {
		return xnode.Period{}, false, xnode.ErrConflict
	}
	var dependency error
	if current.Status == "open" {
		if current.ClosureGeneration == math.MaxInt64 {
			dependency = fmt.Errorf("closure_generation_exhausted: %w", xnode.ErrAccessAccountingPending)
		} else {
			current, err = scanXNodePeriod(tx.QueryRowContext(ctx, `UPDATE xnode_periods SET status='closing',ended_at=$2,
 closure_generation=closure_generation+1 WHERE id=$1 RETURNING `+xnodePeriodColumns, current.ID, stamp(time.Now().UTC())))
		}
	} else if current.Status != "closing" && current.Status != "closed" {
		dependency = fmt.Errorf("period_review_required: %w", xnode.ErrAccessAccountingPending)
	}
	if err != nil {
		return xnode.Period{}, false, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE outbox_jobs SET payload=jsonb_set(payload::jsonb,
 '{nativePeriodCursor}',to_jsonb($2::text),true)::text WHERE id=$1`, jobID, current.ID); err != nil {
		return xnode.Period{}, false, err
	}
	if err = tx.Commit(); err != nil {
		return xnode.Period{}, false, err
	}
	return current, true, dependency
}
