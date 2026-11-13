package app

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestNativeIdentityProvisioningCarriesCapturedGeneration(t *testing.T) {
	t.Parallel()
	f, ctx := newNativePurchaseFixture(t), context.Background()
	if _, err := f.store.DB().ExecContext(ctx, `UPDATE users SET account_generation=1 WHERE id=$1`, f.user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.DB().ExecContext(ctx, `UPDATE xnode_identities SET credentials_cipher='',subscription_cipher='',subscription_hash=NULL,current_period_id=NULL,access_disabled=1 WHERE user_id=$1`, f.user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.provider.native.EnsureIdentity(ctx, f.user); !errors.Is(err, database.ErrAccountUnavailable) {
		t.Fatal("old owner snapshot provisioned revived principal", err)
	}
	identity, err := f.provider.native.IdentityForUser(ctx, f.user.ID)
	if err != nil || identity.CredentialsCipher != "" || !identity.AccessDisabled || identity.CurrentPeriodID != "" {
		t.Fatal("rejected provisioning changed retained principal", identity, err)
	}
	current, err := f.store.UserByID(ctx, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	identity, err = f.provider.native.EnsureIdentity(ctx, current)
	if err != nil || identity.CredentialsCipher == "" || identity.SubscriptionCipher == "" || identity.AccessDisabled || identity.CurrentPeriodID != "" {
		t.Fatal("current owner could not provision dormant credentials", identity, err)
	}
}
