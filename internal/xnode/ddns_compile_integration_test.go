package xnode_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestStoredUnusableDDNSDoesNotBlockReconnectOrAuthorization(t *testing.T) {
	f := controlFixture(t, true)
	ctx := context.Background()
	global, err := f.service.Global(ctx)
	if err != nil {
		t.Fatal(err)
	}
	global.Providers["external"] = wire.Provider{Type: "cloudflare"}
	if err := f.service.SaveGlobal(ctx, f.user.ID, global); err != nil {
		t.Fatal(err)
	}
	stored := xnode.NodeSettings{DDNS: []wire.DDNSJob{
		{ID: "empty-source", Provider: "external", Zone: "example.com", Name: "@"},
		{ID: "overflow-ttl", Provider: "external", Zone: "example.com", Name: "@", IPv4: "8.8.8.8", TTLSeconds: 1 << 31},
		{ID: "working", Provider: "external", Zone: "example.com", Name: "@", IPv4: "8.8.8.8"},
	}}
	// Inject a document accepted before the stricter DDNS execution contract.
	// Production writes must still reject it through SaveNode.
	raw, err := json.Marshal(stored)
	if err != nil {
		t.Fatal(err)
	}
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	node, err := f.store.XNodeNode(ctx, f.node.ID)
	if err != nil {
		t.Fatal(err)
	}
	node.SettingsCipher, err = vault.Encrypt("xnode.node."+node.ID, string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.SaveXNodeNode(ctx, node); err != nil {
		t.Fatal(err)
	}
	originalCipher := node.SettingsCipher
	if _, err := f.service.SaveNode(ctx, node, stored); !errors.Is(err, xnode.ErrInvalid) {
		t.Fatal("new node input accepted an unusable DDNS job", err)
	}
	compiled, err := f.service.Compile(ctx, node.ID)
	if err != nil || len(compiled.Users) != 1 || len(compiled.DDNS) != 1 || compiled.DDNS[0].ID != "working" {
		t.Fatal("stored utility failure blocked current native users or valid sibling", compiled, err)
	}
	if err := compiled.Validate(); err != nil {
		t.Fatal("stored projection relaxed current native wire validation", err)
	}
	server := httptest.NewServer(f.master)
	defer server.Close()
	_, token, err := f.service.Enrollment(ctx, node.ID)
	if err != nil {
		t.Fatal(err)
	}
	socket := agentSocket(t, server, token, fixtureAgent, 0)
	if err := socket.WriteJSON(wire.New("state.request", "", 0, map[string]any{})); err != nil {
		t.Fatal(err)
	}
	baseline := readFrame(t, socket, "state.sync")
	if err := wire.Decode(baseline.Data, &compiled); err != nil || len(compiled.Users) != 1 || len(compiled.DDNS) != 1 {
		t.Fatal("full baseline omitted authorized users or valid DDNS", err)
	}
	if err := f.store.SetXNodeAccessDisabled(ctx, f.identity.ProxyUserID, true); err != nil {
		t.Fatal(err)
	}
	if err := f.master.Sync(ctx, node.ID); err != nil {
		t.Fatal("old utility failure blocked current authorization", err)
	}
	for {
		frame := readFrame(t, socket, "state.sync")
		if err := wire.Decode(frame.Data, &compiled); err != nil {
			t.Fatal(err)
		}
		if len(compiled.Users) == 0 {
			baseline = frame
			break
		}
	}
	_ = socket.Close()
	socket = agentSocket(t, server, token, fixtureAgent, baseline.Revision)
	if err := socket.WriteJSON(wire.New("state.request", "", 0, map[string]any{})); err != nil {
		t.Fatal(err)
	}
	frame := readFrame(t, socket, "state.sync")
	if err := wire.Decode(frame.Data, &compiled); err != nil || len(compiled.Users) != 0 || len(compiled.DDNS) != 1 || compiled.DDNS[0].ID != "working" {
		t.Fatal("reconnect failed to publish current authorization and valid DDNS", err)
	}
	logs, err := f.master.Logs(ctx, node.ID)
	if err != nil || len(logs.Entries) != 1 {
		t.Fatal("reconnect discarded or duplicated the current master diagnostic", logs, err)
	}
	current, settings, err := f.service.Node(ctx, node.ID)
	if err != nil || current.SettingsCipher != originalCipher || !reflect.DeepEqual(settings.DDNS, stored.DDNS) {
		t.Fatal("runtime omission changed encrypted utility input required for correction", err)
	}
}
