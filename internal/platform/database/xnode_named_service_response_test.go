package database

import (
	"context"
	"testing"
)

func TestNamedServiceResponseFailureRollsBackMutation(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	identity, access := namedCadenceFixture()
	saved, err := store.SaveXNodeNamedService(ctx, identity, access)
	if err != nil {
		t.Fatal(err)
	}
	// A retained malformed timestamp makes the public identity response fail
	// validation. That failure must precede the mutation's commit.
	if _, err = store.DB().ExecContext(ctx, `UPDATE xnode_identities SET created_at='invalid-retained-timestamp' WHERE proxy_user_id=$1`, saved.ProxyUserID); err != nil {
		t.Fatal(err)
	}
	changed := saved
	changed.CredentialsCipher = "replacement-credentials"
	access.LimitBytes, access.Disabled = 2000, true
	if _, err = store.SaveXNodeNamedService(ctx, changed, access); err == nil {
		t.Fatal("malformed retained response was accepted")
	}
	var cipher string
	var disabled int
	if err = store.DB().QueryRowContext(ctx, `SELECT credentials_cipher,access_disabled FROM xnode_identities WHERE proxy_user_id=$1`, saved.ProxyUserID).Scan(&cipher, &disabled); err != nil {
		t.Fatal(err)
	}
	if cipher != saved.CredentialsCipher || disabled != 0 {
		t.Fatal("failed response committed replacement credentials or authorization")
	}
	period, err := store.XNodePeriod(ctx, saved.CurrentPeriodID)
	if err != nil || period.LimitBytes != 1000 {
		t.Fatal("failed response committed replacement quota", period, err)
	}
	if _, err = store.DB().ExecContext(ctx, `UPDATE xnode_identities SET created_at=$1 WHERE proxy_user_id=$2`, stamp(saved.CreatedAt), saved.ProxyUserID); err != nil {
		t.Fatal(err)
	}
	current, err := store.SaveXNodeNamedService(ctx, changed, access)
	if err != nil || current.CredentialsCipher != changed.CredentialsCipher || !current.AccessDisabled || current.CurrentPeriodID != saved.CurrentPeriodID {
		t.Fatal("valid edit did not return its committed identity", err)
	}
}
