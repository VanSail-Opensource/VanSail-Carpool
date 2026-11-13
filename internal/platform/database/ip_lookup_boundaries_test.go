package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestIPLookupAllowanceUsesExactPaidBoundary(t *testing.T) {
	for _, name := range []string{"last_nanosecond", "expired", "future_start", "exact_start"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			store, _, owner, purchase, at := ipLookupFixture(t, 2)
			now := at.Truncate(time.Second).Add(time.Hour)
			from, until := now.Add(-time.Hour), now.Add(time.Hour)
			wantQuota := name == "last_nanosecond" || name == "exact_start"
			switch name {
			case "last_nanosecond":
				until = now.Add(time.Nanosecond)
			case "expired":
				until = now
			case "future_start":
				from = now.Add(time.Nanosecond)
			case "exact_start":
				from = now
			}
			if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET valid_from=$1,valid_until=$2 WHERE id=$3`, stamp(from), stamp(until), purchase); err != nil {
				t.Fatal(err)
			}
			quote, err := store.IPLookupQuote(ctx, owner, "8.8.8.8", false, now)
			if err != nil || quote.UseQuota != wantQuota {
				t.Fatal("paid boundary selected the wrong funding source", quote, err)
			}
			quote.ExpiresAt = now.Add(time.Minute).Unix()
			before, err := store.Balance(ctx, owner)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = store.CreateIPLookupCheck(ctx, owner, "exact-quota", quote, now); err != nil {
				t.Fatal(err)
			}
			after, err := store.Balance(ctx, owner)
			wantCharge := int64(250)
			if wantQuota {
				wantCharge = 0
			}
			if err != nil || before.MinorInt64()-after.MinorInt64() != wantCharge {
				t.Fatal("settlement charged the wrong funding source", before, after, err)
			}
			var used int
			if err := store.DB().QueryRowContext(ctx, `SELECT used FROM ip_lookup_allowances WHERE purchase_id=$1`, purchase).Scan(&used); err != nil || (used == 1) != wantQuota {
				t.Fatal("allowance consumption differs from paid eligibility", used, err)
			}
		})
	}
}

func TestIPLookupLeaseTakeoverAndHeartbeatPreserveNanoseconds(t *testing.T) {
	ctx := context.Background()
	store, service, owner, _, at := ipLookupFixture(t, 0)
	op := submitIP(t, service, owner, "precise-lease", false)
	now := at.Truncate(time.Second)
	if claimed, err := store.ClaimIPLookupLease(ctx, op.ID, "first", now); err != nil || !claimed {
		t.Fatal(claimed, err)
	}
	deadline := now.Add(time.Nanosecond)
	resetDeadline := func() {
		t.Helper()
		if _, err := store.DB().ExecContext(ctx, `UPDATE ip_lookup_reports SET lease_expires_at=$1 WHERE id=(SELECT report_id FROM ip_lookup_checks WHERE operation_id=$2)`, stamp(deadline), op.ID); err != nil {
			t.Fatal(err)
		}
	}
	resetDeadline()
	if claimed, err := store.ClaimIPLookupLease(ctx, op.ID, "second", now); err != nil || claimed {
		t.Fatal("live final nanosecond permitted takeover", claimed, err)
	}
	if err := store.HeartbeatIPLookup(ctx, op.ID, "first", now); err != nil {
		t.Fatal("live final nanosecond rejected heartbeat", err)
	}
	resetDeadline()
	if err := store.HeartbeatIPLookup(ctx, op.ID, "first", deadline); !errors.Is(err, ErrConflict) {
		t.Fatal("expired lease accepted heartbeat", err)
	}
	if claimed, err := store.ClaimIPLookupLease(ctx, op.ID, "second", deadline); err != nil || !claimed {
		t.Fatal("exact expiry prevented takeover", claimed, err)
	}
	if err := store.HeartbeatIPLookup(ctx, op.ID, "first", deadline); !errors.Is(err, ErrConflict) {
		t.Fatal("former owner renewed replacement lease", err)
	}
}
