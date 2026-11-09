package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func seedUsageBreakdownHTTPHourly(t *testing.T, f memberHourlyFixture, owner model.User, id, name string) {
	t.Helper()
	ctx, at := context.Background(), time.Now().UTC().Truncate(time.Minute).Add(-time.Minute)
	store := f.server.deps.Store
	if err := store.SaveXNodeGroup(ctx, xnode.Group{ID: id, Name: name}); err != nil {
		t.Fatal(err)
	}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := store.DB().ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`UPDATE users SET created_at=$2 WHERE id=$1`, owner.ID, at.Add(-24*time.Hour).Format(time.RFC3339Nano))
	exec(`INSERT INTO hourly_enrollments(id,owner_id,account_generation,group_id,activation_key,request_fingerprint,status,created_at,updated_at)
 VALUES($1,$2,$3,$1,$1,$4,'preparing',$5,$5)`, id, owner.ID, owner.AccountGeneration, strings.Repeat("a", 64), at)
	principal, err := store.EnsureXNodeHourlyPrincipal(ctx, id, "private-hourly-credential-cipher")
	if err != nil {
		t.Fatal(err)
	}
	exec(`UPDATE xnode_periods SET started_at=$2,ended_at=$3,status='closed' WHERE id=$1`, principal.PeriodID, at.Format(time.RFC3339Nano), at.Add(30*time.Second).Format(time.RFC3339Nano))
	exec(`UPDATE hourly_enrollments SET status='closed',activated_at=$2,paid_through=$3 WHERE id=$1`, id, at, at.Truncate(time.Hour).Add(time.Hour))
	exec(`INSERT INTO xnode_usage_minutes(owner_id,account_generation,billing_mode,reference_id,proxy_user_id,period_id,node_id,minute,observed_from,observed_through,upload_bytes,download_bytes)
 VALUES($1,$2,'hourly',$3,$4,$5,'shared-physical-node',$6,$6,$6,9007199254740993,7)`, owner.ID, owner.AccountGeneration, id, principal.ProxyUserID, principal.PeriodID, at)
	const charge = int64(9007199254740993)
	exec(`INSERT INTO ledger_entries(id,user_id,delta_txb_minor,balance_after,kind,reference_id,created_at)
 VALUES($1,$2,$3,0,'hourly_charge',$1,$4)`, id+"-debit", owner.ID, -charge, at.Format(time.RFC3339Nano))
	exec(`INSERT INTO hourly_settlements(id,enrollment_id,hour_start,charged_from,paid_until,hourly_price_minor,charged_minor,offer_revision,policy_version,ledger_id,created_at)
 VALUES($1,$2,$3,$4,$5,$6,$6,1,$7,$1,$4)`, id+"-debit", id, at.Truncate(time.Hour), at, at.Truncate(time.Hour).Add(time.Hour), charge, strings.Repeat("b", 64))
}

func TestUsageBreakdownHTTPHourlySquadsRemainOwnerScopedAndExact(t *testing.T) {
	f := newUsageBreakdownHTTPFixture(t, false)
	other, _, err := f.server.deps.Store.UpsertTelegramUser(context.Background(), model.TelegramProfile{ID: 80307}, false)
	if err != nil {
		t.Fatal(err)
	}
	seedUsageBreakdownHTTPHourly(t, f, f.user, "owned-hourly", "Named hourly squad")
	seedUsageBreakdownHTTPHourly(t, f, other, "foreign-hourly", "Foreign private squad")
	w := f.request(t, "GET", "/api/v1/usage/breakdown?ownerId="+other.ID, "", "", true)
	var result model.UsageBreakdown
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &result) != nil || len(result.HourlySquads) != 1 || len(result.Nodes) != 0 {
		t.Fatalf("hourly projection failed: %d %s", w.Code, w.Body.String())
	}
	squad := result.HourlySquads[0]
	if squad.GroupID != "owned-hourly" || squad.Name != "Named hourly squad" || squad.UploadBytes != "9007199254740993" || squad.DownloadBytes != "7" || squad.CostMinor != (model.RationalMinor{Numerator: "9007199254740993", Denominator: "1"}) || result.Cost.ActualMinor != "9007199254740993" {
		t.Fatal("hourly bytes or immutable charge rounded/misattributed", result)
	}
	for _, secret := range []string{"foreign-hourly", "Foreign private squad", "shared-physical-node", "proxyUserId", "periodId", "accountGeneration", "private-hourly-credential-cipher"} {
		if strings.Contains(w.Body.String(), secret) {
			t.Fatalf("private attribution detail leaked: %s", secret)
		}
	}
	if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("hourly projection became cacheable", w.Header())
	}
}
