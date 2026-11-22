package database

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestPAYGFundingPrefilterPreservesCurrentPriceAndPrepaidBoundaries(t *testing.T) {
	s, owner, _, account := paygBillingFixture(t)
	ctx, now := context.Background(), time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `INSERT INTO pricing_product_settings
 (product_type,product_id,fixed_price_minor,updated_by,updated_at)
 VALUES('payg',$1,100,'fixture',$2)`, account.GroupID, stamp(now)); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name                   string
		balance, credit, price int64
		dynamic                bool
		want                   bool
	}{
		{"empty", 0, 0, 1, false, false},
		{"small_fixed_exact", 1, 0, 1, false, true},
		{"below_fixed", 1, 0, 2, false, false},
		{"dynamic_retains_account_price", 1, 0, 1, true, false},
		{"dynamic_exact", 100, 0, 1, true, true},
		{"prepaid_empty_wallet", 0, 1, 200, false, true},
		{"prepaid_below_price", 1, 1, 200, false, true},
		{"debt_with_prepaid", -1, 1, 1, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := s.db.ExecContext(ctx, `UPDATE balances SET txb_minor=$1 WHERE user_id=$2`, test.balance, owner.UserID); err != nil {
				t.Fatal(err)
			}
			if _, err := s.db.ExecContext(ctx, `UPDATE xnode_payg_accounts SET credit_bytes=$1,credit_started_at=$2 WHERE proxy_user_id=$3`, test.credit, stamp(now), account.ProxyUserID); err != nil {
				t.Fatal(err)
			}
			if _, err := s.db.ExecContext(ctx, `UPDATE squad_product_overrides SET payg_price_minor=$1 WHERE group_id=$2`, test.price, account.GroupID); err != nil {
				t.Fatal(err)
			}
			mode := "FIXED"
			if test.dynamic {
				mode = "DYNAMIC"
			}
			if _, err := s.db.ExecContext(ctx, `UPDATE pricing_product_settings SET dynamic_enabled=$1,purchase_pricing_mode=$2 WHERE product_type='payg' AND product_id=$3`, boolInt(test.dynamic), mode, account.GroupID); err != nil {
				t.Fatal(err)
			}
			full, err := s.XNodePAYGAccount(ctx, account.ProxyUserID)
			if err != nil {
				t.Fatal(err)
			}
			got, err := s.XNodePAYGFunding(ctx, account.NodeID)
			if err != nil {
				t.Fatal(err)
			}
			want := []xnode.PAYGFunding{}
			if test.want {
				want = append(want, full.Funding())
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("fresh funding differs at financial boundary: got=%+v want=%+v", got, want)
			}
		})
	}
}
