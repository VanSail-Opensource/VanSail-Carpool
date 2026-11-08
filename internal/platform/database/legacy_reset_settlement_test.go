package database

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"testing"
	"time"
)

// This fixture represents a debit committed by a retired release; production has
// no reset admission API. Financial settlement remains replay safe.
func seedLegacyTrafficReset(t *testing.T, s *Store, input providerops.CreateInput, amount int64, now time.Time) providerops.Operation {
	t.Helper()
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	id, err := insertMemberOperationTx(ctx, tx, input, now)
	if err != nil {
		t.Fatal(err)
	}
	balance, err := changeBalanceTx(ctx, tx, input.OwnerUserID, -amount, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = insertLedgerTx(ctx, tx, input.OwnerUserID, -amount, balance, "traffic_reset_debit", id, "historical reset", now); err != nil {
		t.Fatal(err)
	}
	operation, err := scanProviderOperation(tx.QueryRowContext(ctx, providerOperationSelect+` WHERE id=$1`, id))
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	paid, err := s.LegacyTrafficResetPaid(ctx, id, input.OwnerUserID)
	if err != nil || !paid {
		t.Fatal(paid, err)
	}
	paid, err = s.LegacyTrafficResetPaid(ctx, id, "another-owner")
	if err != nil || paid {
		t.Fatal("cross-owner obligation", paid, err)
	}
	return operation
}
