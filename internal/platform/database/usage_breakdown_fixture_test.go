package database

import (
	"context"
	"database/sql"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func usageBreakdownFixture(t *testing.T) (*Store, model.User, time.Time) {
	t.Helper()
	s := newTestStore(t)
	u := createTestUser(t, s, 89881)
	at := time.Now().UTC().Truncate(time.Minute).Add(-5 * time.Minute)
	usageBreakdownExec(t, s, `UPDATE users SET created_at=$2 WHERE id=$1`, u.ID, stamp(at.Add(-96*time.Hour)))
	if err := s.SaveXNodeConfiguration(context.Background(), xnode.Configuration{ID: "breakdown-config", Name: "Breakdown", DocumentCipher: "opaque"}); err != nil {
		t.Fatal(err)
	}
	return s, u, at
}

func usageBreakdownExec(t *testing.T, s *Store, query string, args ...any) {
	t.Helper()
	if _, err := s.DB().ExecContext(context.Background(), query, args...); err != nil {
		t.Fatal(err)
	}
}

// Fixtures construct immutable source facts directly. Projection tests do not
// replace the independent wallet, journal-admission or native-barrier tests.
func usageBreakdownPurchase(t *testing.T, s *Store, u model.User, id string, price int64, seconds *int64, at time.Time) {
	t.Helper()
	combo := saveTestCombo(t, s, id, price, 30)
	usageBreakdownExec(t, s, `INSERT INTO purchases(id,user_id,combo_id,charged_txb_minor,gross_price_txb_minor,valid_from,valid_until,status,created_at,updated_at,purchased_term_seconds)
 VALUES($1,$2,$3,$4,$4,$5,$6,'active',$5,$5,$7)`, id, u.ID, combo.ID, price, stamp(at.Add(-time.Hour)), stamp(at.Add(24*time.Hour)), seconds)
}

func usageBreakdownRaw(t *testing.T, s *Store, u model.User, mode, ref, node string, up, down int64, at time.Time) {
	t.Helper()
	principal, period := int64(1), ref
	if mode == "hourly" {
		var owner string
		var generation int64
		err := s.DB().QueryRowContext(context.Background(), `SELECT owner_id,account_generation,COALESCE(proxy_user_id,0),COALESCE(period_id,'') FROM hourly_enrollments WHERE id=$1`, ref).Scan(&owner, &generation, &principal, &period)
		if err == sql.ErrNoRows {
			// Raw-only projection fixtures need a legitimate scoped principal,
			// without creating a debit. Invalid/orphan evidence uses explicit SQL.
			usageBreakdownExec(t, s, `INSERT INTO hourly_enrollments(id,owner_id,account_generation,group_id,activation_key,request_fingerprint,status,created_at,updated_at) VALUES($1,$2,$3,$1,$1,$4,'preparing',$5,$5)`, ref, u.ID, u.AccountGeneration, strings.Repeat("a", 64), at)
			p, createErr := s.EnsureXNodeHourlyPrincipal(context.Background(), ref, "opaque")
			if createErr != nil {
				t.Fatal(createErr)
			}
			principal, period = p.ProxyUserID, p.PeriodID
		} else if err != nil || owner != u.ID || generation != u.AccountGeneration || principal <= 0 || period == "" {
			t.Fatal("hourly raw fixture must reference its real owner/generation/principal", ref, err)
		}
	}
	usageBreakdownExec(t, s, `INSERT INTO xnode_usage_minutes(owner_id,account_generation,billing_mode,reference_id,proxy_user_id,period_id,node_id,minute,observed_from,observed_through,upload_bytes,download_bytes)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$9,$10,$11)`, u.ID, u.AccountGeneration, mode, ref, principal, period, node, at.Truncate(time.Minute), at, up, down)
}

func usageBreakdownPAYGAccount(t *testing.T, s *Store, u model.User, node string, at time.Time) xnode.PAYGAccount {
	t.Helper()
	ctx := context.Background()
	if err := s.SaveXNodeNode(ctx, xnode.Node{ID: node, Name: node, ConfigID: "breakdown-config", Enabled: true, MultiplierFP: xnode.MultiplierScale, SettingsCipher: "opaque"}); err != nil {
		t.Fatal(err)
	}
	i, err := s.SaveXNodeIdentity(ctx, xnode.Identity{ServiceName: "member-payg-" + node, CredentialsCipher: "opaque"})
	if err != nil {
		t.Fatal(err)
	}
	p := xnode.Period{ID: "period-" + node, ProxyUserID: i.ProxyUserID, StartedAt: at, Strategy: "PAYG", Status: "open"}
	if err = s.SaveXNodePeriod(ctx, p); err != nil {
		t.Fatal(err)
	}
	a := xnode.PAYGAccount{UserID: u.ID, AccountGeneration: u.AccountGeneration, ProxyUserID: i.ProxyUserID, GroupID: "group-" + node, NodeID: node, PeriodID: p.ID, UnitPriceMinor: 1}
	if err = s.SaveXNodePAYGAccount(ctx, a); err != nil {
		t.Fatal(err)
	}
	return a
}

func usageBreakdownLedger(t *testing.T, s *Store, owner, id, kind string, amount int64, at time.Time) {
	t.Helper()
	usageBreakdownExec(t, s, `INSERT INTO ledger_entries(id,user_id,delta_txb_minor,balance_after,kind,reference_id,created_at) VALUES($1,$2,$3,0,$4,$1,$5)`, id, owner, -amount, kind, stamp(at))
}

func usageBreakdownPAYGCharge(t *testing.T, s *Store, a xnode.PAYGAccount, id string, first, blocks, price int64, at time.Time) {
	t.Helper()
	usageBreakdownLedger(t, s, a.UserID, id, "payg_gb", blocks*price, at)
	usageBreakdownExec(t, s, `INSERT INTO xnode_payg_charges(ledger_id,proxy_user_id,first_block,blocks,unit_price_minor) VALUES($1,$2,$3,$4,$5)`, id, a.ProxyUserID, first, blocks, price)
}

func usageBreakdownRational(t *testing.T, value model.RationalMinor) *big.Rat {
	t.Helper()
	v, ok := new(big.Rat).SetString(value.Numerator + "/" + value.Denominator)
	if !ok || v.Sign() < 0 || v.Num().String() != value.Numerator || v.Denom().String() != value.Denominator {
		t.Fatal("noncanonical rational", value)
	}
	return v
}

func assertUsageBreakdownConservation(t *testing.T, value model.UsageBreakdown) {
	t.Helper()
	sum := usageBreakdownRational(t, value.Cost.UnallocatedMinor)
	sum.Add(sum, usageBreakdownRational(t, value.Omitted.CostMinor))
	for _, node := range value.Nodes {
		sum.Add(sum, usageBreakdownRational(t, node.CostMinor))
	}
	for _, squad := range value.HourlySquads {
		sum.Add(sum, usageBreakdownRational(t, squad.CostMinor))
	}
	if sum.Cmp(usageBreakdownRational(t, value.Cost.KnownTotalMinor)) != 0 {
		t.Fatal("entry/unallocated costs lost exact value", sum, value.Cost)
	}
}

func usageBreakdownNode(t *testing.T, value model.UsageBreakdown, id string) model.UsageBreakdownNode {
	t.Helper()
	for _, node := range value.Nodes {
		if node.NodeID == id {
			return node
		}
	}
	t.Fatal(fmt.Sprintf("missing node %q", id))
	return model.UsageBreakdownNode{}
}
