package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestActiveControlOverlapUsesExactExpiry(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name       string
		at, expiry time.Duration
		conflict   bool
	}{
		{"fractional future overlap", 0, 500 * time.Millisecond, true},
		{"nanosecond future overlap", 0, time.Nanosecond, true},
		{"whole expiry before fractional admission", 500 * time.Millisecond, 0, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, store := context.Background(), newTestStore(t)
			now := time.Date(2030, 10, 8, 0, 0, 0, 0, time.UTC)
			combo := saveTestCombo(t, store, "Exact control", 100, 30)
			user, current := createAdminWorkflowPurchase(t, store, 49120, combo, now)
			other := createMemberOperationPurchase(t, store, user.ID, combo.ID, "overlap", now)
			if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET status='active',valid_from=$1,valid_until=$2 WHERE id=$3`,
				stamp(now.Add(-time.Hour)), stamp(now.Add(test.expiry)), other.ID); err != nil {
				t.Fatal(err)
			}
			tx, err := store.DB().BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback() }()
			_, err = activeControlPurchaseTx(ctx, tx, user.ID, current.ID, now.Add(test.at))
			if test.conflict && !errors.Is(err, ErrConflict) || !test.conflict && err != nil {
				t.Fatalf("control error = %v, want conflict %v", err, test.conflict)
			}
		})
	}
}

func TestMemberQueueSelectionAndRebasePreserveNanoseconds(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2030, 10, 8, 0, 0, 0, 0, time.UTC)
	combo := saveTestCombo(t, store, "Exact queue", 100, 30)
	user, _ := createAdminWorkflowPurchase(t, store, 49121, combo, now)
	later := createMemberOperationPurchase(t, store, user.ID, combo.ID, "created-first", now)
	earlier := createMemberOperationPurchase(t, store, user.ID, combo.ID, "created-second", now.Add(time.Minute))
	boundary := now.Add(24 * time.Hour)
	for index, id := range []string{earlier.ID, later.ID} {
		from := boundary.Add(time.Duration(index+1) * time.Nanosecond)
		if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET valid_from=$1,valid_until=$2 WHERE id=$3`, stamp(from), stamp(from.Add(time.Hour)), id); err != nil {
			t.Fatal(err)
		}
	}
	for _, mode := range []string{"early activation selection", "refund selection", "rebase boundary"} {
		t.Run(mode, func(t *testing.T) {
			tx, err := store.DB().BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback() }()
			switch mode {
			case "early activation selection":
				selected, err := queuedControlPurchaseTx(ctx, tx, user.ID, earlier.ID)
				if err != nil || selected.ID != earlier.ID {
					t.Fatalf("first exact queued term = %s, %v", selected.ID, err)
				}
			case "refund selection":
				id, err := activateRefundSuccessorTx(ctx, tx, user.ID, now)
				if err != nil || id != earlier.ID {
					t.Fatalf("refund successor = %s, %v", id, err)
				}
			case "rebase boundary":
				if err := shiftRefundQueueTx(ctx, tx, user.ID, boundary.Add(2*time.Nanosecond), now, now); err != nil {
					t.Fatal(err)
				}
				var raw string
				if err := tx.QueryRowContext(ctx, `SELECT valid_from FROM purchases WHERE id=$1`, earlier.ID).Scan(&raw); err != nil {
					t.Fatal(err)
				}
				if from, err := parseStamp(raw); err != nil || !from.Equal(boundary.Add(time.Nanosecond)) {
					t.Fatalf("earlier term moved across exact rebase boundary: %s, %v", raw, err)
				}
			}
		})
	}
}
