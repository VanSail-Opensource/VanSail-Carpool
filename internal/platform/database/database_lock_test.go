package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestWriteTransactionsReserveBeforeReadingWhileReadOnlyKeepsWALConcurrency(t *testing.T) {
	for _, readOnly := range []bool{false, true} {
		t.Run(map[bool]string{false: "writer", true: "read-only"}[readOnly], func(t *testing.T) {
			store := newTestStore(t)
			ctx := context.Background()
			if _, err := store.db.ExecContext(ctx, `CREATE TABLE lock_fixture(value INTEGER); INSERT INTO lock_fixture VALUES(0)`); err != nil {
				t.Fatal(err)
			}
			other, err := store.db.Conn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer other.Close()
			if _, err = other.ExecContext(ctx, `PRAGMA busy_timeout(1)`); err != nil {
				t.Fatal(err)
			}
			tx, err := store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: readOnly})
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			var value int
			if err = tx.QueryRowContext(ctx, `SELECT value FROM lock_fixture`).Scan(&value); err != nil {
				t.Fatal(err)
			}
			_, err = other.ExecContext(ctx, `UPDATE lock_fixture SET value=1`)
			if readOnly {
				if err != nil {
					t.Fatal("read-only snapshot blocked independent writer", err)
				}
				if err = tx.QueryRowContext(ctx, `SELECT value FROM lock_fixture`).Scan(&value); err != nil || value != 0 {
					t.Fatalf("read snapshot changed: %d, %v", value, err)
				}
				return
			}
			var coded interface{ Code() int }
			if !errors.As(err, &coded) || coded.Code()&255 != 5 {
				t.Fatalf("concurrent writer entered reserved transaction: %v", err)
			}
			if _, err = tx.ExecContext(ctx, `UPDATE lock_fixture SET value=2`); err != nil {
				t.Fatal("reserved writer could not upgrade snapshot", err)
			}
			if err = tx.Commit(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
