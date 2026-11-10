package xnode

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type nodeSettingsRepository struct {
	Repository
	node Node
}

func (r *nodeSettingsRepository) XNodeNode(context.Context, string) (Node, error) {
	return r.node, nil
}
func (r *nodeSettingsRepository) XNodeNodes(context.Context) ([]Node, error) {
	return []Node{r.node}, nil
}

func encryptedNodeSettings(t *testing.T, raw string) (*Service, *nodeSettingsRepository) {
	t.Helper()
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := vault.Encrypt("xnode.node.node", raw)
	if err != nil {
		t.Fatal(err)
	}
	repo := &nodeSettingsRepository{node: Node{ID: "node", Name: "Node", ConfigID: "config", SettingsCipher: cipher}}
	return NewService(repo, cacheTestSettings{}, vault, nil, nil), repo
}

func TestEncryptedNodeSettingsDiscardRetiredBandwidthWithoutChangingDesiredState(t *testing.T) {
	raw := `{"bandwidth":{"normalMbps":100,"burstMbps":200,"afterSeconds":30,"resetSeconds":60},"dns":{"mode":"xray","redirect":true,"upstreams":["127.0.0.1"]},"ddns":[{"id":"wan","provider":"builtin:panel","interface":"eth0","ipv4":"auto"}],"certificates":{"local":{"config":{"cert_mode":"file","cert_file":"cert.pem","key_file":"key.pem"}}},"inboundCertificates":{"paid":"local"},"firewall":{"enabled":true,"protect":true,"ssh_ports":[1022],"tcp_ports":[443]}}`
	s, repo := encryptedNodeSettings(t, raw)
	original := repo.node.SettingsCipher
	_, settings, err := s.Node(context.Background(), "node")
	if err != nil {
		t.Fatal(err)
	}
	want := NodeSettings{
		DNS:                 wire.DNSPolicy{Mode: "xray", Redirect: true, Upstreams: []string{"127.0.0.1"}},
		DDNS:                []wire.DDNSJob{{ID: "wan", Provider: wire.BuiltinDDNSProvider, Interface: "eth0", IPv4: "auto"}},
		DDNSBindings:        map[string]DDNSHostBinding{},
		Certificates:        map[string]wire.Certificate{"local": {Config: wire.CertConfig{CertMode: "file", CertFile: "cert.pem", KeyFile: "key.pem"}}},
		InboundCertificates: map[string]string{"paid": "local"},
		Firewall:            wire.FirewallPolicy{Enabled: true, Protect: true, SSHPorts: []int{1022}, TCPPorts: []int{443}},
	}
	if !reflect.DeepEqual(settings, want) || repo.node.SettingsCipher != original {
		t.Fatal("reading legacy settings changed current policy or stored ciphertext")
	}
	encoded, err := json.Marshal(settings)
	var fields map[string]json.RawMessage
	if err != nil || json.Unmarshal(encoded, &fields) != nil {
		t.Fatal("encode current settings", err)
	}
	if _, retained := fields["bandwidth"]; retained {
		t.Fatal("retired bandwidth escaped into current settings")
	}
}

// TestEncryptedNodeSettingsStillRejectMalformedState keeps the read path strict
// about documents it cannot trust. Unknown/retired stored keys are historical
// evidence and are tolerated by design; see TestDecodeNodeSettingsIgnoresRetiredStoredFields.
func TestEncryptedNodeSettingsStillRejectMalformedState(t *testing.T) {
	for _, raw := range []string{
		`null`, `[]`, `"settings"`, `0`, `{`, `{} {}`,
		`{"bandwidth":{},"firewall":{"enabled":"true"}}`,
	} {
		t.Run(raw, func(t *testing.T) {
			s, repo := encryptedNodeSettings(t, raw)
			original := repo.node.SettingsCipher
			if _, _, err := s.Node(context.Background(), "node"); err == nil {
				t.Fatal("invalid encrypted state accepted")
			}
			if repo.node.SettingsCipher != original {
				t.Fatal("failed read rewrote desired state")
			}
		})
	}
	s, repo := encryptedNodeSettings(t, `{}`)
	repo.node.ID = "foreign"
	if _, _, err := s.Node(context.Background(), "foreign"); err == nil {
		t.Fatal("settings authenticated for a different node were accepted")
	}
}

func TestSavingLegacyNodeSettingsRemovesOnlyRetiredBandwidth(t *testing.T) {
	s, source := encryptedNodeSettings(t, `{"bandwidth":{"normalMbps":100},"dns":{"mode":"xray"},"firewall":{"enabled":true,"protect":true,"ssh_ports":[1022]}}`)
	repo := &ddnsBindingTestRepository{nodes: map[string]Node{"node": source.node}, bindings: map[string]DDNSHostBinding{}}
	s.repo = repo
	ctx := context.Background()
	node, settings, err := s.Node(ctx, "node")
	if err != nil {
		t.Fatal(err)
	}
	saved, err := s.SaveNode(ctx, node, settings)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := s.vault.Decrypt("xnode.node.node", saved.SettingsCipher)
	var fields map[string]json.RawMessage
	if err != nil || json.Unmarshal([]byte(raw), &fields) != nil {
		t.Fatal("read saved settings", err)
	}
	if _, retained := fields["bandwidth"]; retained {
		t.Fatal("save retained retired bandwidth")
	}
	_, restored, err := s.Node(ctx, "node")
	if err != nil || !reflect.DeepEqual(restored, settings) {
		t.Fatal("save changed current settings", err)
	}
}

func TestAutomaticFirewallLeasesReadLegacyEncryptedNodeSettings(t *testing.T) {
	s, repo := encryptedNodeSettings(t, `{"bandwidth":{"normalMbps":100},"firewall":{"enabled":true}}`)
	master := NewMaster(s, nil)
	now := time.Now().UTC()
	s.now = func() time.Time { return now }
	if !master.automaticLeases.observe([]AutomaticLeaseObservation{{IP: "8.8.8.8", Node: "node", At: now}}, now, time.Minute) {
		t.Fatal("fixture lease was not observed")
	}
	original := repo.node.SettingsCipher
	page, err := s.AutomaticFirewallLeases(context.Background(), "", 1, 10)
	if err != nil || len(page.Items) != 1 || page.Items[0].PendingNodes != 1 {
		t.Fatal("legacy settings prevented lease delivery status", err)
	}
	if repo.node.SettingsCipher != original {
		t.Fatal("lease read rewrote desired state")
	}
}
