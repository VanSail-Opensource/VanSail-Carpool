package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func namedCadenceFixture() (xnode.Identity, xnode.ServiceAccess) {
	return xnode.Identity{ServiceName: "quota-service", CredentialsCipher: "original-credentials", SubscriptionHash: []byte("service-subscription"), SubscriptionCipher: "subscription"},
		xnode.ServiceAccess{LimitBytes: 1000, StartedAt: time.Now().UTC().Add(-time.Hour), ExpiresAt: time.Now().UTC().Add(time.Hour), GroupIDs: []string{"fixture-group"}}
}

func TestNamedServiceDatabaseRejectsNewCadenceBeforeWriting(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	identity, access := namedCadenceFixture()
	for _, strategy := range []string{"DAY", "WEEK", "MONTH", "MONTH_ROLLING"} {
		access.Strategy = strategy
		if _, err := store.SaveXNodeNamedService(ctx, identity, access); !errors.Is(err, xnode.ErrInvalid) {
			t.Fatal("database admitted new cadence", strategy, err)
		}
	}
	var count int
	if err := store.DB().QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM xnode_identities)+(SELECT COUNT(*) FROM xnode_periods)`).Scan(&count); err != nil || count != 0 {
		t.Fatal("rejected creation left state", count, err)
	}
	access.Strategy = ""
	saved, err := store.SaveXNodeNamedService(ctx, identity, access)
	if err != nil {
		t.Fatal(err)
	}
	period, err := store.XNodePeriod(ctx, saved.CurrentPeriodID)
	if err != nil || period.Strategy != "NO_RESET" {
		t.Fatal("omitted database cadence was not defaulted", period, err)
	}
}

func TestNamedServiceDatabaseRejectsCadenceChangeAtomically(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	identity, access := namedCadenceFixture()
	saved, err := store.SaveXNodeNamedService(ctx, identity, access)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.DB().ExecContext(ctx, `UPDATE xnode_periods SET used_bytes=400 WHERE id=$1`, saved.CurrentPeriodID); err != nil {
		t.Fatal(err)
	}
	changed := saved
	changed.CredentialsCipher = "rejected-credentials"
	access.Strategy, access.LimitBytes, access.Disabled = "DAY", 2000, true
	if _, err = store.SaveXNodeNamedService(ctx, changed, access); !errors.Is(err, xnode.ErrInvalid) {
		t.Fatal("database enabled cadence", err)
	}
	current, err := store.XNodeIdentity(ctx, saved.ProxyUserID)
	if err != nil || current.CredentialsCipher != saved.CredentialsCipher || current.AccessDisabled {
		t.Fatal("rejection changed encrypted terms or authorization", err)
	}
	period, err := store.XNodePeriod(ctx, saved.CurrentPeriodID)
	if err != nil || period.UsedBytes != 400 || period.LimitBytes != 1000 || period.Strategy != "NO_RESET" {
		t.Fatal("rejection changed quota or historical usage", period, err)
	}
}

func TestNamedServiceDatabasePreservesProvenLegacyCadence(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	identity, access := namedCadenceFixture()
	saved, err := store.SaveXNodeNamedService(ctx, identity, access)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.DB().ExecContext(ctx, `UPDATE xnode_periods SET strategy='DAY',used_bytes=77 WHERE id=$1`, saved.CurrentPeriodID); err != nil {
		t.Fatal(err)
	}
	for _, strategy := range []string{"", "DAY"} {
		access.Strategy, access.LimitBytes = strategy, 2000
		if _, err = store.SaveXNodeNamedService(ctx, saved, access); err != nil {
			t.Fatal("database rejected unchanged legacy cadence", err)
		}
	}
	for _, strategy := range []string{"NO_RESET", "MONTH"} {
		access.Strategy = strategy
		if _, err = store.SaveXNodeNamedService(ctx, saved, access); !errors.Is(err, xnode.ErrInvalid) {
			t.Fatal("database replaced legacy cadence", strategy, err)
		}
	}
	period, err := store.XNodePeriod(ctx, saved.CurrentPeriodID)
	if err != nil || period.Strategy != "DAY" || period.UsedBytes != 77 || period.LimitBytes != 2000 {
		t.Fatal("grandfathered accounting changed", period, err)
	}
	if _, err = store.DB().ExecContext(ctx, `UPDATE xnode_identities SET current_period_id=NULL WHERE proxy_user_id=$1`, saved.ProxyUserID); err != nil {
		t.Fatal(err)
	}
	saved.CurrentPeriodID, access.Strategy = "", "DAY"
	if _, err = store.SaveXNodeNamedService(ctx, saved, access); !errors.Is(err, xnode.ErrInvalid) {
		t.Fatal("database inferred cadence without authoritative period", err)
	}
	current, err := store.XNodeIdentity(ctx, saved.ProxyUserID)
	if err != nil || current.CurrentPeriodID != "" || current.CredentialsCipher != saved.CredentialsCipher {
		t.Fatal("periodless rejection changed legacy identity", err)
	}
}
