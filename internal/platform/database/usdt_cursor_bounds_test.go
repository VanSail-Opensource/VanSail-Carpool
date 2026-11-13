package database

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usdt"
)

func TestUSDTOverlargeCursorPreservesPriorBatchAndRejectsLegacyLoad(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newTestStore(t)
	owner := createTestUser(t, s, 88102)
	order := createUSDTTestOrder(t, s, owner.ID, "1.000")
	quote := reserveUSDTTestQuote(t, s, order)
	prior := usdt.Cursor{Value: "100", Hash: "prior-block"}
	for _, test := range []struct {
		name   string
		cursor usdt.Cursor
	}{
		{name: "value", cursor: usdt.Cursor{Value: strings.Repeat("v", usdt.MaxCursorBytes+1), Hash: prior.Hash}},
		{name: "hash", cursor: usdt.Cursor{Value: "101", Hash: strings.Repeat("h", usdt.MaxCursorHashBytes+1)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := s.SaveUSDTBatch(ctx, quote.Network, usdt.Batch{Cursor: prior}); err != nil {
				t.Fatal(err)
			}
			transfer := usdtTestTransfer(quote, "oversized-"+test.name, time.Now().UTC())
			if err := s.SaveUSDTBatch(ctx, quote.Network, usdt.Batch{Transfers: []usdt.Transfer{transfer}, Cursor: test.cursor}); !errors.Is(err, usdt.ErrCapacity) {
				t.Fatal("oversized batch was accepted", err)
			}
			got, err := s.USDTCursor(ctx, quote.Network)
			if err != nil || got != prior {
				t.Fatal("rejected batch advanced committed recovery cursor", got, err)
			}
			var transfers int
			if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM usdt_transfers`).Scan(&transfers); err != nil || transfers != 0 {
				t.Fatal("rejected batch inserted receiving evidence", transfers, err)
			}
			if _, err := s.db.ExecContext(ctx, `UPDATE usdt_scan_cursors SET value=$1,hash=$2 WHERE network=$3 AND token=$4`, test.cursor.Value, test.cursor.Hash, quote.Network, quote.Token); err != nil {
				t.Fatal(err)
			}
			if got, err := s.USDTCursor(ctx, quote.Network); !errors.Is(err, usdt.ErrCapacity) || got != (usdt.Cursor{}) {
				t.Fatal("oversized retained cursor was materialized or accepted", err)
			}
		})
	}
}
