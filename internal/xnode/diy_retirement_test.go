package xnode_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestDIYRetirementResealsOnlyClientDocumentWithoutCharging(t *testing.T) {
	f := controlFixture(t, false)
	ctx := context.Background()
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	old := `{"version":1,"overrides":{"fingerprint":"firefox","tcpFastOpen":false,"tcpMptcp":true,"minVersion":"1.2","downloadTcpFastOpen":true},"whitelist":{"enabled":true,"ips":["8.8.8.8"]},"splitProfiles":[{"id":"one","uploadEndpointId":"up","downloadEndpointId":"down","mode":"auto"}]}`
	cipher, err := vault.Encrypt("xnode.diy."+f.user.ID, old)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.DB().ExecContext(ctx, `INSERT INTO xnode_member_diy(user_id,revision,cipher) VALUES(?,7,?)`, f.user.ID, cipher); err != nil {
		t.Fatal(err)
	}
	var ledgerBefore int
	if err = f.store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries`).Scan(&ledgerBefore); err != nil {
		t.Fatal(err)
	}
	if err = f.service.RetireDIYOptions(ctx); err != nil {
		t.Fatal(err)
	}
	doc, err := f.store.XNodeDIY(ctx, f.user.ID)
	if err != nil || doc.Revision != 8 || doc.Cipher == cipher {
		t.Fatal(doc, err)
	}
	raw, err := vault.Decrypt("xnode.diy."+f.user.ID, doc.Cipher)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"minVersion", "downloadTcpFastOpen", "tcpMptcp"} {
		if strings.Contains(raw, key) {
			t.Fatal("retired option persisted", key)
		}
	}
	var settings xnode.DIYSettings
	if err = json.Unmarshal([]byte(raw), &settings); err != nil || settings.Version != 2 || settings.Overrides.Fingerprint == nil || !settings.Whitelist.Enabled || len(settings.Whitelist.IPs) != 1 || len(settings.SplitProfiles) != 1 || settings.SplitProfiles[0].DownloadEndpointID != "down" {
		t.Fatal(settings, err)
	}
	if err = f.service.RetireDIYOptions(ctx); err != nil {
		t.Fatal(err)
	}
	again, err := f.store.XNodeDIY(ctx, f.user.ID)
	if err != nil || again.Cipher != doc.Cipher || again.Revision != doc.Revision {
		t.Fatal("retirement not idempotent", again, err)
	}
	var ledgerAfter int
	if err = f.store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries`).Scan(&ledgerAfter); err != nil || ledgerBefore != ledgerAfter {
		t.Fatal("automatic cleanup charged member", ledgerBefore, ledgerAfter, err)
	}
}
