package xnode_test

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestOwnedHostNamesKeepCredentialsAndRestorePublishedName(t *testing.T) {
	f := controlFixture(t, true)
	ctx := context.Background()
	endpoint, err := f.service.SaveEndpoint(ctx, xnode.Endpoint{NodeID: f.node.ID, InboundTag: "paid", Name: "Published {price}", Address: "host.example", Port: 443, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	proxies, err := f.service.MemberProxies(ctx, f.user.ID)
	if err != nil || len(proxies) != 1 {
		t.Fatal(proxies, err)
	}
	before := proxies[0]
	if before.Name != endpoint.Name {
		t.Fatal("published host name includes generated identity", before.Name)
	}
	saved, err := f.service.SaveOwnedHostName(ctx, f.user.ID, endpoint.ID, "My {price}")
	if err != nil || saved.Name != "My 1.0x" {
		t.Fatal(saved, err)
	}
	exported, err := f.service.ExportProxiesForUser(ctx, f.user.ID, proxies)
	if err != nil || exported[0].Name != saved.Name || exported[0].ID != before.ID || exported[0].PrincipalID != before.PrincipalID {
		t.Fatal(exported, err)
	}
	link, err := exported[0].NativeLink()
	parsed, parseErr := url.Parse(link)
	if err != nil || parseErr != nil || parsed.Fragment != saved.Name {
		t.Fatal(link, err, parseErr)
	}
	clash, err := exported[0].Clash()
	if err != nil || clash["name"] != saved.Name {
		t.Fatal(clash, err)
	}
	sing, err := exported[0].SingBox()
	if err != nil || sing["tag"] != saved.Name {
		t.Fatal(sing, err)
	}
	if proxies[0].Name != before.Name {
		t.Fatal("mutated shared inventory")
	}
	other, _, err := f.store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 99188, FirstName: "Other"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.EnsureIdentity(ctx, other); err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.SaveOwnedHostName(ctx, other.ID, endpoint.ID, "Other name"); !errors.Is(err, xnode.ErrNotFound) {
		t.Fatal("inaccessible existing host accepted", err)
	}
	otherNames, err := f.store.HostNameOverrides(ctx, other.ID)
	if err != nil || len(otherNames) != 0 {
		t.Fatal("owner preference leaked", otherNames, err)
	}
	if _, err = f.service.SaveOwnedHostName(ctx, f.user.ID, "other-owner-host", "no"); !errors.Is(err, xnode.ErrNotFound) {
		t.Fatal("unowned host accepted", err)
	}
	if _, err = f.service.SaveOwnedHostName(ctx, f.user.ID, endpoint.ID, strings.Repeat("名", 101)); !errors.Is(err, xnode.ErrInvalid) {
		t.Fatal("unbounded name accepted", err)
	}
	restored, err := f.service.SaveOwnedHostName(ctx, f.user.ID, endpoint.ID, "")
	if err != nil || restored.Name != "Published 1.0x" || restored.NameOverride != "" {
		t.Fatal(restored, err)
	}
	names, err := f.store.HostNameOverrides(ctx, f.user.ID)
	if err != nil || len(names) != 0 {
		t.Fatal("reset retained override", names, err)
	}
}
