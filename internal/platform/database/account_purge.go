package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"
)

// CompleteAccountPurge retains exact financial references and a non-login subject.
func (s *Store) retryAttempt_CompleteAccountPurge(ctx context.Context, id string, generation int64, now time.Time) error {

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var state, role string
	var current int64
	if err = tx.QueryRowContext(ctx, `SELECT account_status,account_generation,role FROM users WHERE id=$1`, id).Scan(&state, &current, &role); err != nil {
		return err
	}
	if current != generation || state != "purging" || role == "admin" {
		return nil
	}
	// The admission helper acquires health -> user -> candidate locks. No
	// destructive step may use a stale role, bot identity or final probe.
	valid, err := s.accountCleanupAdmissionTx(ctx, tx, id, generation, now)
	if err != nil {
		return err
	}
	if !valid {
		return ErrCleanupAdmissionRequired
	}
	if err = fenceAccountPurgeOperationsTx(ctx, tx, id, now); err != nil {
		return err
	}
	if dependency, err := accountPurgeDependency(ctx, tx, id, now); err != nil {
		return err
	} else if dependency != "" {
		return fmt.Errorf("%s: %w", dependency, ErrConflict)
	}
	if err = accountPurgeIPControlsReadyTx(ctx, tx, id, generation); err != nil {
		return err
	}
	if err = accountPurgeExemptionsReadyTx(ctx, tx, id); err != nil {
		return err
	}
	var pending bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM outbox_jobs WHERE kind<>'account_purge' AND status='processing' AND `+accountJobTargets+`)`, id, id, id, id, id).Scan(&pending); err != nil {
		return err
	}
	if pending {
		return ErrConflict
	}
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM xnode_periods p JOIN xnode_identities i ON i.proxy_user_id=p.proxy_user_id WHERE p.status<>'closed' AND (i.user_id=$1 OR i.proxy_user_id IN (SELECT proxy_user_id FROM xnode_payg_accounts WHERE user_id=$2) OR i.proxy_user_id IN (SELECT proxy_user_id FROM hourly_enrollments WHERE owner_id=$1))) OR EXISTS(SELECT 1 FROM hourly_credit_reservations WHERE owner_id=$1 AND (final_proof_digest='' OR final_recovery AND unknown_bytes>0))`, id, id).Scan(&pending); err != nil {
		return err
	}
	if pending {
		return ErrConflict
	}
	// Provider/native row locks can wait beyond the final probe's lifetime.
	// Recheck the database clock immediately before financial or data removal.
	valid, err = s.accountCleanupAdmissionTx(ctx, tx, id, generation, now)
	if err != nil {
		return err
	}
	if !valid {
		return ErrCleanupAdmissionRequired
	}
	if err = closeAccountBalanceTx(ctx, tx, id, generation, now); err != nil {
		return err
	}
	if err = archiveAccountAcceptanceTx(ctx, tx, id, generation, now); err != nil {
		return err
	}
	if err = archiveAccountCreditTx(ctx, tx, id, generation, now); err != nil {
		return err
	}
	if err = purgeAccountMaterialTx(ctx, tx, id, now); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE users SET account_status='purged',telegram_first_name='',telegram_last_name='',telegram_username='',proxy_user_id=NULL,group_joined=0,channel_joined=0,inviter_id=NULL,recovery_reason='',policy_accepted_at=NULL,accepted_agreement_revision=0,notification_locale='en',display_currency='TXB',pm_blocked=0,pm_muted=0,updated_at=$1 WHERE id=$2 AND account_generation=$3 AND account_status='purging' AND role='user'`, stamp(now), id, generation)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		return ErrConflict
	}
	if err = insertAuditTx(ctx, tx, fmt.Sprintf("account-purge-complete:%s:%d", id, generation), nil, "account.purged", "user", id, `{"retained":"immutable_accounting"}`, now); err != nil {
		return err
	}
	return tx.Commit()
}

func closeAccountBalanceTx(ctx context.Context, tx *sql.Tx, id string, generation int64, now time.Time) error {
	balance, err := balanceTx(ctx, tx, id)
	if err != nil {
		return err
	}
	// A minimum signed balance needs two representable compensating entries.
	for step := 0; balance != 0; step++ {
		delta := -balance
		if balance == math.MinInt64 {
			delta = math.MaxInt64
		}
		balance, err = adjustBalanceTx(ctx, tx, id, delta, now)
		if err != nil {
			return err
		}
		if _, err = insertLedgerTx(ctx, tx, id, delta, balance, "account_closure", fmt.Sprintf("%s:%d:%d", id, generation, step), "account closed after verified reachability grace", now); err != nil {
			return err
		}
	}
	return nil
}

func archiveAccountAcceptanceTx(ctx context.Context, tx *sql.Tx, id string, generation int64, now time.Time) error {
	var accepted sql.NullString
	var revision int
	if err := tx.QueryRowContext(ctx, `SELECT policy_accepted_at,accepted_agreement_revision FROM users WHERE id=$1`, id).Scan(&accepted, &revision); err != nil {
		return err
	}
	detail, err := json.Marshal(map[string]any{"acceptedAt": accepted.String, "revision": revision, "generation": generation})
	if err != nil {
		return err
	}
	if err = insertAuditTx(ctx, tx, fmt.Sprintf("account-purge-consent:%s:%d", id, generation), nil, "account.archived_consent", "user", id, string(detail), now); err != nil {
		return err
	}
	var embyID, remote string
	var price int64
	err = tx.QueryRowContext(ctx, `SELECT id,COALESCE(remote_user_id,''),setup_price_txb_minor FROM emby_accounts WHERE user_id=$1`, id).Scan(&embyID, &remote, &price)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	detail, err = json.Marshal(map[string]any{"accountId": embyID, "remoteId": remote, "setupPriceMinor": price})
	if err != nil {
		return err
	}
	return insertAuditTx(ctx, tx, "account-purge-emby:"+embyID, nil, "account.archived_emby", "user", id, string(detail), now)
}
