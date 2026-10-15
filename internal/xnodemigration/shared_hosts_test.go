package xnodemigration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

type sharedHostSource struct {
	*cutoverSource
	topology Snapshot
}

func (s sharedHostSource) EndpointTopology(context.Context, SourceConfig) (Snapshot, error) {
	return s.topology, nil
}

func sharedHostTopology(method string, explicit []string) Snapshot {
	host, _ := json.Marshal(map[string]any{"uuid": "shared", "address": "rr.proxy.example", "port": 443, "nodes": explicit, "inbound": map[string]any{"configProfileInboundUuid": "inbound"}})
	config, _ := json.Marshal(map[string]any{"inbounds": []any{map[string]any{"tag": "paid", "protocol": "shadowsocks", "settings": map[string]any{"method": method}}}})
	return Snapshot{Profiles: []remnawave.MigrationProfile{{UUID: "profile", Config: config}}, Nodes: []Node{
		{ID: "node", ProfileID: "profile", Inbounds: []remnawave.NodeInbound{{UUID: "inbound", Tag: "paid"}}},
		{ID: "peer", ProfileID: "profile", Inbounds: []remnawave.NodeInbound{{UUID: "inbound", Tag: "paid"}}},
	}, Hosts: []json.RawMessage{host}}
}

func TestSharedSS2022PreflightKeepsSourceServing(t *testing.T) {
	for _, scenario := range []string{"explicit", "implicit", "unknown-peer", "separate-hosts-same-address", "compatible-chacha", "isolated-host", "unrelated-draft"} {
		t.Run(scenario, func(t *testing.T) {
			s, repo, source, _, events := migrationCutoverFixture(t)
			topology := sharedHostTopology("2022-blake3-aes-256-gcm", []string{"node", "peer"})
			switch scenario {
			case "implicit":
				topology = sharedHostTopology("2022-blake3-aes-128-gcm", nil)
			case "unknown-peer":
				topology = sharedHostTopology("2022-blake3-aes-256-gcm", []string{"node", "unknown"})
			case "compatible-chacha":
				topology = sharedHostTopology("chacha20-ietf-poly1305", []string{"node", "peer"})
			case "unrelated-draft":
				topology = sharedHostTopology("2022-blake3-aes-256-gcm", []string{"node"})
				topology.Profiles = append(topology.Profiles, remnawave.MigrationProfile{UUID: "unrelated", Config: json.RawMessage("[]")})
			case "isolated-host":
				topology = sharedHostTopology("2022-blake3-aes-256-gcm", []string{"node"})
			case "separate-hosts-same-address":
				topology = sharedHostTopology("2022-blake3-aes-256-gcm", []string{"node"})
				topology.Hosts = append(topology.Hosts, json.RawMessage(`{"uuid":"peer-host","address":"RR.PROXY.EXAMPLE.","port":443,"nodes":["peer"],"inbound":{"configProfileInboundUuid":"inbound"}}`))
			}
			s.source = sharedHostSource{cutoverSource: source, topology: topology}
			_, err := s.Cutover(context.Background(), "node")
			if scenario == "compatible-chacha" || scenario == "isolated-host" || scenario == "unrelated-draft" {
				if err != nil || repo.state.Nodes[0].Phase != "native" {
					t.Fatalf("safe handover=%+v %v", repo.state, err)
				}
				return
			}
			if !errors.Is(err, ErrSharedEndpoint) || !errors.Is(err, xnode.ErrConflict) || repo.state.Nodes[0].Phase != "legacy" {
				t.Fatalf("unsafe handover=%+v %v", repo.state, err)
			}
			for _, event := range *events {
				if event == "source:disable" {
					t.Fatal("shared source was interrupted")
				}
			}
			expectedIssues := 1
			if scenario == "separate-hosts-same-address" {
				expectedIssues = 2
			}
			preview := inspectSnapshot(topology)
			if len(preview.Issues) != expectedIssues {
				t.Fatalf("missing preview warning=%+v", preview)
			}
			for _, issue := range preview.Issues {
				if issue.Code != "SHARED_SS2022_HOST" || !issue.Blocking {
					t.Fatalf("incorrect host warning=%+v", issue)
				}
			}
		})
	}
}

func TestLastLegacyPeerCanJoinAlreadyNativeSS2022Host(t *testing.T) {
	topology := sharedHostTopology("2022-blake3-aes-256-gcm", []string{"node", "peer"})
	issues, err := sharedSS2022Hosts(topology, "node", map[string]bool{"peer": true})
	if err != nil || len(issues) != 0 {
		t.Fatalf("last legacy peer cannot complete handover=%v %v", issues, err)
	}
}
