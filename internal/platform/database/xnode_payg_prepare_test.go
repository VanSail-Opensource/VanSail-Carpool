package database

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestPAYGPreparationCommitsOnePrincipalPeriodAndAssociation(t *testing.T) {
	s, input := paygPreparationFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	const workers = 8
	before, err := s.Balance(ctx, input.OwnerID)
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		proxy  int64
		period string
		err    error
	}
	done := make(chan result, workers)
	for n := range workers {
		go func(n int) {
			value := input
			value.PeriodID = fmt.Sprintf("payg-concurrent-%d", n)
			value.CredentialsCipher = fmt.Sprintf("cipher-%d", n)
			a, e := s.PrepareXNodePAYGAccount(ctx, value)
			done <- result{a.ProxyUserID, a.PeriodID, e}
		}(n)
	}
	var proxy int64
	var period string
	for range workers {
		r := <-done
		if r.err != nil {
			t.Fatal(r.err)
		}
		if proxy == 0 {
			proxy, period = r.proxy, r.period
		}
		if r.proxy != proxy || r.period != period {
			t.Fatal("competing preparation changed native tuple", r, proxy, period)
		}
	}
	var identities, periods, participants int
	if err = s.db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM xnode_identities WHERE service_name=$1),(SELECT COUNT(*) FROM xnode_periods WHERE proxy_user_id=$2),(SELECT COUNT(*) FROM xnode_period_nodes WHERE period_id=$3)`, input.ServiceName, proxy, period).Scan(&identities, &periods, &participants); err != nil || identities != 1 || periods != 1 || participants != 1 {
		t.Fatal("partial native preparation", identities, periods, participants, err)
	}
	stored, err := s.XNodePeriod(ctx, period)
	if err != nil || stored.Strategy != "PAYG" || stored.UsedBytes != 0 {
		t.Fatal("PAYG accounting semantics changed", stored, err)
	}
	after, err := s.Balance(ctx, input.OwnerID)
	if err != nil || after != before {
		t.Fatal("preparation charged wallet", before, after, err)
	}
}

func TestPAYGPreparationRollbackCannotLeavePrivateOrphans(t *testing.T) {
	s, input := paygPreparationFixture(t)
	ctx := context.Background()
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE xnode_payg_accounts ADD CONSTRAINT reject_test_preparation CHECK(group_id<>'atomic-payg')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PrepareXNodePAYGAccount(ctx, input); err == nil {
		t.Fatal("injected association failure was accepted")
	}
	var identities, periods int
	if err := s.db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM xnode_identities WHERE service_name=$1),(SELECT COUNT(*) FROM xnode_periods WHERE id=$2)`, input.ServiceName, input.PeriodID).Scan(&identities, &periods); err != nil || identities != 0 || periods != 0 {
		t.Fatal("failed association leaked credentials/period", identities, periods, err)
	}
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE xnode_payg_accounts DROP CONSTRAINT reject_test_preparation`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PrepareXNodePAYGAccount(ctx, input); err != nil {
		t.Fatal("exact retry failed after clean rollback", err)
	}
}
