package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// ReleaseAccountPurgeExemptions retires only this account's allowances through
// the existing versioned firewall delivery queue. Pending delivery still commits
// the release; offline members never become a fabricated successful receipt.
func (s *Store) ReleaseAccountPurgeExemptions(ctx context.Context, target model.AccountPurge, jobID string) (bool, error) {
	return hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (bool, error) {
		_, allowed, err := s.accountNativePurgeGuardTx(ctx, tx, target, jobID)
		if err != nil || !allowed {
			return false, err
		}
		// Sweep/grant paths acquire the clock before mutating exemption rows.
		if err = lockAccountPurgeFirewallClock(ctx, tx, true); err != nil {
			return false, err
		}
		version, unreleased, err := accountPurgeExemptionVersionsTx(ctx, tx, target.UserID)
		if err != nil {
			return false, err
		}
		if unreleased {
			issued, e := firewallVersion(ctx, tx)
			if e != nil {
				return false, e
			}
			if _, e = tx.ExecContext(ctx, `UPDATE xnode_firewall_exemptions SET released_version=$2 WHERE user_id=$1 AND released_version=0`, target.UserID, issued); e != nil {
				return false, e
			}
			version = max(version, issued)
		}
		if version == 0 {
			return true, nil
		}
		if err = queueFirewallFleetTx(ctx, tx, time.Now().UTC()); err != nil {
			return false, err
		}
		return accountPurgeFirewallDeliveredTx(ctx, tx, version)
	})
}

// Caller already holds cleanup health/user/evidence/provider fences. Keep rows
// until the removal revision is confirmed; otherwise deletion erases the delta.
func accountPurgeExemptionsReadyTx(ctx context.Context, tx *sql.Tx, owner string) error {
	if err := lockAccountPurgeFirewallClock(ctx, tx, false); err != nil {
		return err
	}
	version, unreleased, err := accountPurgeExemptionVersionsTx(ctx, tx, owner)
	if err != nil {
		return err
	}
	delivered := false
	if !unreleased {
		delivered, err = accountPurgeFirewallDeliveredTx(ctx, tx, version)
	}
	if err != nil {
		return err
	}
	if !delivered {
		return fmt.Errorf("ACCOUNT_PURGE_FIREWALL_EXEMPTION_PENDING: %w", xnode.ErrAccessAccountingPending)
	}
	return nil
}

func lockAccountPurgeFirewallClock(ctx context.Context, tx *sql.Tx, exclusive bool) error {
	lock := " FOR SHARE"
	if exclusive {
		lock = " FOR UPDATE"
	}
	var version uint64
	return tx.QueryRowContext(ctx, `SELECT version FROM xnode_firewall_clock WHERE singleton=1`+lock).Scan(&version)
}

func accountPurgeExemptionVersionsTx(ctx context.Context, tx *sql.Tx, owner string) (uint64, bool, error) {
	rows, err := tx.QueryContext(ctx, `SELECT released_version FROM xnode_firewall_exemptions WHERE user_id=$1 ORDER BY id FOR UPDATE`, owner)
	if err != nil {
		return 0, false, err
	}
	var version uint64
	unreleased := false
	for rows.Next() {
		var v uint64
		if err = rows.Scan(&v); err != nil {
			rows.Close()
			return 0, false, err
		}
		unreleased = unreleased || v == 0
		version = max(version, v)
	}
	return version, unreleased, finishCompileRows(rows)
}

func accountPurgeFirewallDeliveredTx(ctx context.Context, tx *sql.Tx, version uint64) (bool, error) {
	if version == 0 {
		return true, nil
	}
	var pending bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM xnode_firewall_members WHERE active=1 AND (applied<$1 OR force_sync<>0))`, version).Scan(&pending)
	return !pending, err
}
