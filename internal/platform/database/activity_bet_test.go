package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

func TestActivityBetAtomicOutcomeAndReplay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		roll        int64
		wantWon     bool
		wantPayout  int64
		wantBalance string
		wantLedger  int
	}{
		{name: "win returns total multiplier", roll: 0, wantWon: true, wantPayout: 200, wantBalance: "1100", wantLedger: 2},
		{name: "loss keeps stake debited", roll: 9_999, wantWon: false, wantPayout: 0, wantBalance: "900", wantLedger: 1},
	}
	for index, test := range tests {
		index, test := index, test
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			store := newTestStore(t)
			user := createTestUser(t, store, 31_000+int64(index))
			if _, err := store.AdjustBalance(ctx, user.ID, 1_000, "activity-seed-"+test.name, "seed", time.Now()); err != nil {
				t.Fatalf("AdjustBalance(): %v", err)
			}
			game, err := store.SaveActivityGame(ctx, activity.GameInput{Name: "Coin", Icon: "coin", Enabled: true, WinChanceBPS: 5_000,
				MinimumStakeMinor: 100, MaximumStakeMinor: 500, ReturnMultiplierBPS: 20_000}, time.Now())
			if err != nil {
				t.Fatalf("SaveActivityGame(): %v", err)
			}
			played, err := store.PlaceActivityBet(ctx, user.ID, game.ID, 100, "bet-key", fixedActivityRandom{value: test.roll}, time.Now())
			if err != nil {
				t.Fatalf("PlaceActivityBet(): %v", err)
			}
			if played.Won != test.wantWon || played.PayoutMinor != test.wantPayout {
				t.Fatalf("bet = won %t payout %d, want %t/%d", played.Won, played.PayoutMinor, test.wantWon, test.wantPayout)
			}
			replayed, err := store.PlaceActivityBet(ctx, user.ID, game.ID, 100, "bet-key", fixedActivityRandom{value: 1}, time.Now())
			if err != nil || !replayed.Replayed || replayed.ID != played.ID {
				t.Fatalf("replay = (%+v, %v)", replayed, err)
			}
			balance, err := store.Balance(ctx, user.ID)
			if err != nil || balance.Minor != test.wantBalance {
				t.Fatalf("Balance() = (%s, %v), want %s", balance.Minor, err, test.wantBalance)
			}
			var ledgerCount int
			if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE reference_id=?`, played.ID).Scan(&ledgerCount); err != nil {
				t.Fatalf("count bet ledger: %v", err)
			}
			if ledgerCount != test.wantLedger {
				t.Fatalf("bet ledger count = %d, want %d", ledgerCount, test.wantLedger)
			}
		})
	}
}
