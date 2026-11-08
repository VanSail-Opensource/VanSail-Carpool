package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestNativePurgeReviewDependencyAdvancesWithoutLosingUncertainty(t *testing.T) {
	s, target, jobID, identity := nativePurgeFixture(t)
	ctx := context.Background()
	_, err := s.db.ExecContext(ctx, `INSERT INTO xnode_periods(id,proxy_user_id,started_at,limit_bytes,strategy,status)
 VALUES('000-review',$1,$2,0,'NO_RESET','review'),('001-next',$1,$2,0,'NO_RESET','open')`, identity.ProxyUserID, stamp(time.Now().UTC()))
	if err != nil {
		t.Fatal(err)
	}
	page, _, err := s.AccountNativePurgePage(ctx, target, jobID, nil)
	if err != nil || len(page) == 0 || page[0].ID != "000-review" {
		t.Fatal(page, err)
	}
	p, active, err := s.AdvanceAccountNativePurge(ctx, target, jobID, page[0])
	if !active || !errors.Is(err, xnode.ErrAccessAccountingPending) || p.Status != "review" {
		t.Fatal("review evidence changed or disappeared", p, active, err)
	}
	next, _, err := s.AccountNativePurgePage(ctx, target, jobID, nil)
	if err != nil || len(next) == 0 || next[0].ID != "001-next" {
		t.Fatal("review dependency starved later periods", next, err)
	}
	stored, err := s.XNodePeriod(ctx, "000-review")
	if err != nil || stored.Status != "review" || stored.ClosureGeneration != 0 {
		t.Fatal("uncertain review was overwritten", stored, err)
	}
}
