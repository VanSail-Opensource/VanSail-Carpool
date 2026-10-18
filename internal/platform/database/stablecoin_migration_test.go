package database

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStablecoinUpgradePreservesLegacyCursorFingerprintsAndEvidenceIDs(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "tokens.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	s := NewStore(db)
	user := createTestUser(t, s, 99034)
	q := stablecoinOrder(t, s, user.ID, "usdt.erc20")
	id := saveUSDTTestTransfer(t, s, usdtTestTransfer(q, "legacy-evidence", time.Now().UTC()))
	legacy, err := migrations.ReadFile("migrations/072_builtin_usdt.sql")
	if err != nil {
		t.Fatal(err)
	}
	transferSchema := "CREATE TABLE usdt_transfers" + strings.Split(strings.Split(string(legacy), "CREATE TABLE usdt_transfers")[1], "CREATE TABLE usdt_scan_cursors")[0]
	_, err = db.ExecContext(ctx, `CREATE TABLE retained_evidence AS SELECT id,network,token,recipient,atomic_amount,amount,tx_hash,event_id,occurred_at,order_id,status,reason,received_at FROM usdt_transfers;
 DROP TABLE usdt_transfers;`+transferSchema+`
 INSERT INTO usdt_transfers SELECT * FROM retained_evidence; DROP TABLE retained_evidence;
 DROP TABLE usdt_scan_cursors;
 CREATE TABLE usdt_scan_cursors(network TEXT PRIMARY KEY,value TEXT NOT NULL,hash TEXT NOT NULL,updated_at TEXT NOT NULL);
 INSERT INTO usdt_scan_cursors VALUES('ethereum','21004567','canonical-hash','2026-10-18T00:00:00Z');
 DELETE FROM schema_migrations WHERE version='085_stablecoin_tokens.sql';`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s = NewStore(db)
	cursor, err := s.USDTCursor(ctx, "usdt.erc20")
	if err != nil || cursor.Value != "21004567" || cursor.Hash != "canonical-hash" {
		t.Fatal("legacy progress reset", cursor, err)
	}
	circle, err := s.USDTCursor(ctx, "usdc.erc20")
	if err != nil || circle.Value != "" {
		t.Fatal("USDC inherited USDT progress", circle, err)
	}
	retained, err := s.USDTQuote(ctx, q.OrderID)
	if err != nil || retained != q {
		t.Fatal("permanent quote changed", retained, err)
	}
	evidence, err := scanUSDTTransfer(db.QueryRowContext(ctx, usdtTransferSelect+" WHERE id=?", id))
	if err != nil || evidence.ID != id || evidence.Token != q.Token {
		t.Fatal("evidence identity changed", evidence, err)
	}
	watch, err := s.USDTWatchAddresses(ctx, "usdt.erc20")
	if err != nil || len(watch) != 1 || watch[0] != q.Recipient {
		t.Fatal("issued quote watcher lost", watch, err)
	}
	watch, err = s.USDTWatchAddresses(ctx, "usdc.erc20")
	if err != nil || len(watch) != 0 {
		t.Fatal("token watcher contamination", watch, err)
	}
}
