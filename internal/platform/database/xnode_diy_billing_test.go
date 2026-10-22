package database

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/iplookup"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func diyBillingInput(user, key string) providerops.CreateInput {
	return providerops.CreateInput{ActorUserID: user, OwnerUserID: user, Kind: xnode.DIYOperationKind, IdempotencyKey: key, RequestFingerprint: strings.Repeat("f", 64), Items: []providerops.ItemInput{{Key: "settings", TargetType: "member_diy", TargetID: user}}}
}

func diyBillingQuote(t *testing.T, s *Store, user string, now time.Time) xnode.DIYBillingQuote {
	t.Helper()
	q, err := s.XNodeDIYBillingQuote(context.Background(), user, now)
	if err != nil {
		t.Fatal(err)
	}
	q.ExpiresAt = now.Add(time.Minute).Unix()
	return q
}

func TestDIYAndIPCheckCompeteForOneAllowanceWithoutFallbackCharge(t *testing.T) {
	s, service, user, _, now := ipLookupFixture(t, 1)
	ctx := context.Background()
	quote := diyBillingQuote(t, s, user, now)
	ipQuote, err := service.Quote(ctx, user, "8.8.8.8", false)
	if err != nil {
		t.Fatal(err)
	}
	var errs [2]error
	var group sync.WaitGroup
	group.Add(2)
	go func() {
		defer group.Done()
		_, errs[0] = s.SaveXNodeDIY(ctx, xnode.DIYDocument{UserID: user, Cipher: "encrypted", Billing: &quote}, 0, nil, diyBillingInput(user, "diy-concurrent"), now)
	}()
	go func() {
		defer group.Done()
		_, errs[1] = s.CreateIPLookupCheck(ctx, user, "ip-concurrent", ipQuote, now)
	}()
	group.Wait()
	if (errs[0] == nil) == (errs[1] == nil) {
		t.Fatalf("expected one admission, got %v", errs)
	}
	state, err := s.IPLookupState(ctx, user, now)
	if err != nil || state.Allowance.Remaining != 0 {
		t.Fatal(state, err)
	}
	var charged int
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM ledger_entries WHERE user_id=? AND kind IN('xnode_diy','ip_lookup')`, user).Scan(&charged); err != nil || charged != 0 {
		t.Fatal("silently charged stale free quote", charged, err)
	}
}

func TestDIYBillingRollbackReplayAndFreeUnchangedSave(t *testing.T) {
	for _, scenario := range []string{"quota", "paid", "deployment", "insufficient", "expired", "stale", "unchanged"} {
		t.Run(scenario, func(t *testing.T) {
			quota := 0
			if scenario == "quota" {
				quota = 1
			}
			s, _, user, _, now := ipLookupFixture(t, quota)
			ctx := context.Background()
			if scenario == "insufficient" {
				c := iplookup.DefaultConfig()
				c.LookupFeeTXB = "1000000"
				if err := s.SaveIPLookupSettings(ctx, user, c, nil, nil); err != nil {
					t.Fatal(err)
				}
			}
			q := diyBillingQuote(t, s, user, now)
			if scenario == "expired" {
				q.ExpiresAt = now.Unix()
			}
			if scenario == "stale" {
				q.ConfigHash = "old"
			}
			result := xnode.DIYSaveResult{SettingsRevision: 1, Changed: true}
			doc := xnode.DIYDocument{UserID: user, Cipher: "private-settings", Billing: &q, Result: &result, Unchanged: scenario == "unchanged"}
			doc.NativeChanged = scenario == "deployment"
			if doc.Unchanged {
				result.SettingsRevision = 0
				result.Changed = false
			}
			input := diyBillingInput(user, "save")
			receipt, err := s.SaveXNodeDIY(ctx, doc, 0, nil, input, now)
			if scenario == "insufficient" || scenario == "expired" || scenario == "stale" {
				if scenario == "insufficient" && !errors.Is(err, ErrInsufficientBalance) || scenario != "insufficient" && !errors.Is(err, xnode.ErrConflict) {
					t.Fatal("unexpected rejection", err)
				}
				saved, readErr := s.XNodeDIY(ctx, user)
				if readErr != nil || saved.Revision != 0 || saved.Cipher != "" {
					t.Fatal("failed save persisted", saved, readErr)
				}
				if _, found, _, readErr := s.ReplayXNodeDIY(ctx, user, input.IdempotencyKey, input.RequestFingerprint); readErr != nil || found {
					t.Fatal("failed save leaked receipt", readErr)
				}
				return
			}
			wantStatus := "succeeded"
			if doc.NativeChanged {
				wantStatus = "queued"
			}
			if err != nil || receipt.Status != wantStatus {
				t.Fatal(receipt, err)
			}
			if doc.NativeChanged {
				op, err := s.BeginProviderOperationAttempt(ctx, receipt.ID, now)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = s.CompleteXNodeProviderOperation(ctx, receipt.ID, op.Attempts, providerops.Completion{Status: providerops.StatusSucceeded, ResultJSON: `{"whitelist":true}`}, now); err != nil {
					t.Fatal(err)
				}
			}
			// Lost replies replay before expiry, price, revision or quota checks.
			q.ExpiresAt = 0
			replayed, err := s.SaveXNodeDIY(ctx, doc, 0, nil, input, now.Add(time.Hour))
			if err != nil || replayed.ID != receipt.ID {
				t.Fatal("duplicate charge/receipt", replayed, err)
			}
			got, found, err := s.ReplayXNodeDIYResult(ctx, user, input.IdempotencyKey, input.RequestFingerprint)
			if err != nil || !found || got.SettingsRevision != result.SettingsRevision || got.Changed != result.Changed {
				t.Fatal("lost committed outcome", got, err)
			}
			var count int
			if err = s.db.QueryRow(`SELECT COUNT(*) FROM ledger_entries WHERE user_id=? AND kind='xnode_diy'`, user).Scan(&count); err != nil {
				t.Fatal(err)
			}
			want := 0
			if scenario == "paid" || scenario == "deployment" {
				want = 1
			}
			if count != want {
				t.Fatalf("ledger count %d want %d", count, want)
			}
			saved, err := s.XNodeDIY(ctx, user)
			if err != nil || saved.Revision != result.SettingsRevision {
				t.Fatal(saved, err)
			}
		})
	}
}
