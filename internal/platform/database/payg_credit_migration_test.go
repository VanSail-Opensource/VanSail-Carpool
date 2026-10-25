package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestPAYGCreditMigrationKeepsOriginalChargeTimeAndHistory(t *testing.T) {
	s, _, _, a := paygBillingFixture(t)
	ctx := context.Background()
	charged := time.Now().UTC().Add(-73 * time.Hour)
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = buyPAYGTx(ctx, tx, &a, 1, false, charged); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().ExecContext(ctx, `UPDATE xnode_periods SET used_bytes=? WHERE id=?`, xnode.PAYGStartBytes, a.PeriodID); err != nil {
		t.Fatal(err)
	}
	// Remove only the new operational state to reconstruct the previous PAYG
	// schema. Existing ledger, prices, purchases and raw periods remain intact.
	for _, query := range []string{
		`DROP TABLE xnode_payg_windows`,
		`ALTER TABLE xnode_payg_accounts DROP COLUMN credit_started_at`,
		`ALTER TABLE xnode_payg_accounts DROP COLUMN credit_bytes`,
	} {
		if _, err := s.DB().ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	script, err := migrations.ReadFile("migrations/096_payg_credit_windows.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().ExecContext(ctx, string(script)); err != nil {
		t.Fatal(err)
	}
	value, err := s.XNodePAYGAccount(ctx, a.ProxyUserID)
	if err != nil || value.CreditBytes != xnode.PAYGGB-xnode.PAYGStartBytes || !value.CreditStartedAt.Equal(charged) || value.HasCredit(time.Now()) || value.PurchasedBytes != xnode.PAYGGB || value.BalanceMinor != 900 {
		t.Fatal("upgrade reset expiry, counters or wallet", value, err)
	}
	var count int
	var price int64
	if err := s.DB().QueryRowContext(ctx, `SELECT COUNT(*),MIN(unit_price_minor) FROM xnode_payg_charges WHERE proxy_user_id=?`, a.ProxyUserID).Scan(&count, &price); err != nil || count != 1 || price != 100 {
		t.Fatal("upgrade rewrote financial evidence", count, price, err)
	}
}
