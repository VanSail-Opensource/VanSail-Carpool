package database

import (
	"context"
	"database/sql"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"testing"
)

func TestPostgresSerializableRejectsStaleWriter(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE lock_fixture(id bigint PRIMARY KEY,value bigint);INSERT INTO lock_fixture VALUES(1,0)`); err != nil {
		t.Fatal(err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var level string
	if err = tx.QueryRowContext(ctx, `SHOW transaction_isolation`).Scan(&level); err != nil || level != "serializable" {
		t.Fatal("unsafe write isolation", level, err)
	}
	var value int
	if err = tx.QueryRowContext(ctx, `SELECT value FROM lock_fixture WHERE id=1`).Scan(&value); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.ExecContext(ctx, `UPDATE lock_fixture SET value=1 WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	var conflict *pgconn.PgError
	if _, err = tx.ExecContext(ctx, `UPDATE lock_fixture SET value=2 WHERE id=1`); !errors.As(err, &conflict) || conflict.Code != "40001" {
		t.Fatal("stale writer must retry", err)
	}
	read, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		t.Fatal(err)
	}
	defer read.Rollback()
	if err = read.QueryRowContext(ctx, `SELECT value FROM lock_fixture WHERE id=1`).Scan(&value); err != nil || value != 1 {
		t.Fatal(value, err)
	}
	if _, err = s.db.ExecContext(ctx, `UPDATE lock_fixture SET value=3 WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	if err = read.QueryRowContext(ctx, `SELECT value FROM lock_fixture WHERE id=1`).Scan(&value); err != nil || value != 1 {
		t.Fatal("read snapshot changed", value, err)
	}
}
