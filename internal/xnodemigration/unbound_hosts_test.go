package xnodemigration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestUnboundSourcePlaceholdersDoNotBlockManagedImportOrHandover(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "enabled placeholder", true: "disabled placeholder"}[disabled], func(t *testing.T) {
			placeholder, err := json.Marshal(map[string]any{"uuid": "unbound", "address": "unused.example", "port": 443, "isDisabled": disabled, "nodes": []string{}, "inbound": map[string]any{"configProfileInboundUuid": nil}})
			if err != nil {
				t.Fatal(err)
			}
			snapshot := sharedHostTopology("2022-blake3-aes-256-gcm", []string{"node"})
			snapshot.Hosts = append([]json.RawMessage{placeholder}, snapshot.Hosts...)
			issues, err := sharedSS2022Hosts(snapshot, "node", map[string]bool{})
			if err != nil || len(issues) != 0 {
				t.Fatalf("unbound placeholder blocked isolated handover: %+v, %v", issues, err)
			}
			vault, err := secret.NewVault(make([]byte, 32))
			if err != nil {
				t.Fatal(err)
			}
			repo := &hostImportRepo{}
			service := &Service{repo: repo, vault: vault}
			if err = service.importEndpoints(context.Background(), snapshot, map[string]map[string]string{"node": {"inbound": "paid"}}); err != nil {
				t.Fatal(err)
			}
			if len(repo.items) != 1 || repo.items[0].NodeID != "node" {
				t.Fatal("managed host omitted or placeholder assigned a fabricated node")
			}
			if len(snapshot.Hosts) != 2 {
				t.Fatal("source placeholder was removed from historical snapshot")
			}
		})
	}
}

func TestDeclaredSourceNodeWithoutAnInboundStillFailsClosed(t *testing.T) {
	raw := json.RawMessage(`{"uuid":"unresolved","nodes":["node"],"inbound":{"configProfileInboundUuid":null}}`)
	snapshot := sharedHostTopology("2022-blake3-aes-256-gcm", []string{"node"})
	snapshot.Hosts = append(snapshot.Hosts, raw)
	if _, err := sharedSS2022Hosts(snapshot, "node", nil); !errors.Is(err, xnode.ErrInvalid) {
		t.Fatalf("unresolved declared node accepted: %v", err)
	}
	service := &Service{repo: &hostImportRepo{}}
	if err := service.importEndpoints(context.Background(), Snapshot{Hosts: []json.RawMessage{raw}}, nil); !errors.Is(err, xnode.ErrInvalid) {
		t.Fatalf("unresolved node imported: %v", err)
	}
}
