package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPAYGPreparationWaitsForPurgeAndRejectsOldGeneration(t *testing.T) {
	s, input := paygPreparationFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, input.OwnerID); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, e := s.PrepareXNodePAYGAccount(ctx, input); done <- e }()
	waitNativeIdentityQueryBlocked(t, s, ctx, "FOR UPDATE")
	if _, err = tx.ExecContext(ctx, `UPDATE users SET account_status='purged',account_generation=1 WHERE id=$1`, input.OwnerID); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = <-done; !errors.Is(err, ErrAccountUnavailable) {
		t.Fatal("atomic preparation crossed purge", err)
	}
	if _, err = s.db.ExecContext(ctx, `UPDATE users SET account_status='active' WHERE id=$1`, input.OwnerID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.PrepareXNodePAYGAccount(ctx, input); !errors.Is(err, ErrAccountUnavailable) {
		t.Fatal("old generation restored native account after rejoin", err)
	}
	input.AccountGeneration = 1
	account, err := s.PrepareXNodePAYGAccount(ctx, input)
	if err != nil || account.AccountGeneration != 1 {
		t.Fatal("new generation could not prepare", account, err)
	}
	account.AccountGeneration = 0
	if err = s.SaveXNodePAYGAccount(ctx, account); !errors.Is(err, ErrAccountUnavailable) {
		t.Fatal("bare association bypassed generation fence", err)
	}
}
