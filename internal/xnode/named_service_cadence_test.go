package xnode_test

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// Seed immutable pre-retirement/imported terms without reopening their write API.
func seedLegacyServiceCadence(t *testing.T, f fixture, identity xnode.Identity, access xnode.ServiceAccess) xnode.Identity {
	t.Helper()
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	scope := "xnode.identity.service." + identity.ServiceName
	raw, err := vault.Decrypt(scope, identity.CredentialsCipher)
	if err != nil {
		t.Fatal(err)
	}
	var credentials xnode.Credentials
	if err = json.Unmarshal([]byte(raw), &credentials); err != nil {
		t.Fatal(err)
	}
	credentials.ServiceAccess = &access
	encoded, err := json.Marshal(credentials)
	if err != nil {
		t.Fatal(err)
	}
	identity.CredentialsCipher, err = vault.Encrypt(scope, string(encoded))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	tx, err := f.store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE xnode_identities SET credentials_cipher=$1 WHERE proxy_user_id=$2`, identity.CredentialsCipher, identity.ProxyUserID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE xnode_periods SET strategy=$1 WHERE id=$2`, access.Strategy, identity.CurrentPeriodID); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return identity
}

func TestNewNamedServiceRejectsCadencesWithoutCreatingState(t *testing.T) {
	f := controlFixture(t, false)
	ctx := context.Background()
	access := xnode.ServiceAccess{LimitBytes: 1000, StartedAt: time.Now().UTC(), ExpiresAt: time.Now().Add(time.Hour), GroupIDs: []string{f.group.ID}}
	var beforeIdentities, beforePeriods int
	if err := f.store.DB().QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM xnode_identities),(SELECT COUNT(*) FROM xnode_periods)`).Scan(&beforeIdentities, &beforePeriods); err != nil {
		t.Fatal(err)
	}
	for _, strategy := range []string{"DAY", "WEEK", "MONTH", "MONTH_ROLLING"} {
		access.Strategy = strategy
		if _, err := f.service.SaveNamedService(ctx, "", "new-service", access); !errors.Is(err, xnode.ErrInvalid) {
			t.Fatalf("new %s term accepted: %v", strategy, err)
		}
	}
	var identities, periods int
	if err := f.store.DB().QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM xnode_identities),(SELECT COUNT(*) FROM xnode_periods)`).Scan(&identities, &periods); err != nil || identities != beforeIdentities || periods != beforePeriods {
		t.Fatal("rejected cadence created operational state", identities, periods, err)
	}
	access.Strategy = ""
	created, err := f.service.SaveNamedService(ctx, "", "new-service", access)
	if err != nil || created.Access.Strategy != "NO_RESET" {
		t.Fatal("omitted new cadence did not default to NO_RESET", created, err)
	}
	access.Strategy = "DAY"
	if _, err = f.service.SaveNamedService(ctx, strconv.FormatInt(created.Identity.ProxyUserID, 10), "new-service", access); !errors.Is(err, xnode.ErrInvalid) {
		t.Fatal("cadence enabled on existing non-resetting term", err)
	}
	after, err := f.store.XNodeIdentity(ctx, created.Identity.ProxyUserID)
	if err != nil || after.CredentialsCipher != created.Identity.CredentialsCipher {
		t.Fatal("rejection changed saved credentials", err)
	}
}

func TestNamedServiceEditsPreserveLegacyCadenceAndUsage(t *testing.T) {
	f, identity, access := standaloneFixture(t, 1000, 123, "MONTH_ROLLING")
	ctx := context.Background()
	id := strconv.FormatInt(identity.ProxyUserID, 10)
	for _, strategy := range []string{"", "MONTH_ROLLING"} {
		access.Strategy, access.LimitBytes = strategy, 2000
		item, err := f.service.SaveNamedService(ctx, id, identity.ServiceName, access)
		if err != nil || item.Access.Strategy != "MONTH_ROLLING" {
			t.Fatal("unchanged grandfathered cadence rejected", item, err)
		}
		period, err := f.store.XNodePeriod(ctx, identity.CurrentPeriodID)
		if err != nil || period.Strategy != "MONTH_ROLLING" || period.UsedBytes != 123 || period.LimitBytes != 2000 {
			t.Fatal("edit rewrote grandfathered accounting", period, err)
		}
	}
	before, err := f.store.XNodeIdentity(ctx, identity.ProxyUserID)
	if err != nil {
		t.Fatal(err)
	}
	for _, strategy := range []string{"DAY", "NO_RESET"} {
		access.Strategy = strategy
		if _, err = f.service.SaveNamedService(ctx, id, identity.ServiceName, access); !errors.Is(err, xnode.ErrInvalid) {
			t.Fatal("changed grandfathered cadence accepted", strategy, err)
		}
	}
	after, err := f.store.XNodeIdentity(ctx, identity.ProxyUserID)
	if err != nil || after.CredentialsCipher != before.CredentialsCipher || after.CurrentPeriodID != before.CurrentPeriodID {
		t.Fatal("cadence rejection changed saved legacy term", err)
	}
	if _, err = f.store.DB().ExecContext(ctx, `UPDATE xnode_identities SET current_period_id=NULL WHERE proxy_user_id=$1`, identity.ProxyUserID); err != nil {
		t.Fatal(err)
	}
	access.Strategy = ""
	if _, err = f.service.SaveNamedService(ctx, id, identity.ServiceName, access); !errors.Is(err, xnode.ErrInvalid) {
		t.Fatal("unproven periodless grandfathered write accepted", err)
	}
	after, err = f.store.XNodeIdentity(ctx, identity.ProxyUserID)
	if err != nil || after.CredentialsCipher != before.CredentialsCipher || after.CurrentPeriodID != "" {
		t.Fatal("periodless rejection changed saved legacy term", err)
	}
}
