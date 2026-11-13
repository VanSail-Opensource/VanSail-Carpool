package database

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	domain "github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
	remote "github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/emby"
)

func TestEmbyServerRestoreRebindsProvenCandidateAndCompletesSetup(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	expected := createEmbyRestoreTarget(t, store, 98_932, "restored-account", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	state := &embyRefundHTTPState{policy: domain.Policy{
		"IsDisabled": json.RawMessage("false"), "EnableMediaPlayback": json.RawMessage("true"),
	}}
	state.user = map[string]any{"Id": "0123456789abcdef0123456789abcdef", "Name": expected.CandidateUsername, "Policy": state.policy}
	server := httptest.NewServer(state)
	defer server.Close()
	client, err := remote.NewClient(server.URL, "test-administrator-key", remote.WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	service := domain.NewService(store, client, nil, embyRefundSecrets{})
	if err := service.Provision(ctx, expected.ID); err != nil {
		t.Fatal(err)
	}
	account, err := store.EmbyProvisioningByID(ctx, expected.ID)
	if err != nil || account.Status != domain.StatusActive || account.RemoteUserID != "0123456789abcdef0123456789abcdef" ||
		account.SetupAttempt != expected.SetupAttempt || account.PasswordCiphertext != "" || account.RefundedAt != nil {
		t.Fatalf("server restore did not recover existing setup: %+v, %v", account.Account, err)
	}
	if err := service.Provision(ctx, expected.ID); err != nil {
		t.Fatal("completed restore did not replay", err)
	}
	var count int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events
		WHERE action='emby.identity.restore' AND target_id=$1`, expected.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("restore evidence count=%d error=%v", count, err)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.creates != 0 || state.password != "member-password" {
		t.Fatal("restore created another account or did not apply the retained password")
	}
}
