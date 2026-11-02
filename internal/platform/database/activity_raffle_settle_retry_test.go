package database

import (
	"context"
	"testing"
	"time"
)

func TestSettleRaffleRetryAfterReopen(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	now := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	const drawID = "reopened-raffle"
	_, err := store.DB().ExecContext(ctx, `INSERT INTO activity_lucky_draws
  (id,name,description,kind,status,fee_minor,threshold,keyword,command,revision,created_at,updated_at)
  VALUES($1,$2,$3,'raffle','open',$4,$5,$6,$7,$8,$9,$10)`, drawID, "Reopened", "", 100, 1, "join", "join_draw", 1, stamp(now), stamp(now))
	if err != nil {
		t.Fatal(err)
	}
	settlement, err := store.SettleRaffle(ctx, drawID, fixedActivityRandom{}, now)
	if err != nil || !settlement.Reopened || settlement.Draw.Status != "open" {
		t.Fatalf("SettleRaffle(retry) = (%+v, %v), want reopened", settlement, err)
	}
}
