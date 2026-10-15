package xnodemigration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

type hostImportRepo struct {
	Repository
	items []xnode.Endpoint
}

func (r *hostImportRepo) XNodeEndpoints(context.Context) ([]xnode.Endpoint, error) {
	return r.items, nil
}
func (r *hostImportRepo) SaveXNodeEndpoint(_ context.Context, item xnode.Endpoint) error {
	r.items = append(r.items, item)
	return nil
}

func TestImportUnboundHostAndResumeWithoutLosingACL(t *testing.T) {
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	repo := &hostImportRepo{}
	s := &Service{repo: repo, vault: vault}
	bindings := map[string]map[string]string{"one": {"inbound": "paid"}, "two": {"inbound": "paid"}, "unrelated": {"different": "other"}}
	snapshot := Snapshot{Hosts: []json.RawMessage{json.RawMessage(`{"uuid":"host","remark":"Shared host","address":"proxy.example","port":443,"nodes":[],"inbound":{"configProfileInboundUuid":"inbound"},"internalSquads":{"mode":"ALLOW_ONLY","squads":["owned"]}}`)}}
	if err := s.importEndpoints(context.Background(), snapshot, bindings); err != nil {
		t.Fatal(err)
	}
	if len(repo.items) != 2 {
		t.Fatalf("inbound host relations lost: %+v", repo.items)
	}
	repo.items[0].Address = "operator-edited.proxy.example"
	if err := s.importEndpoints(context.Background(), snapshot, bindings); err != nil {
		t.Fatal(err)
	}
	if repo.items[0].Address != "operator-edited.proxy.example" {
		t.Fatal("resumed import overwrote operator edits")
	}
	if len(repo.items) != 2 || repo.items[0].NodeID != "one" || repo.items[1].NodeID != "two" {
		t.Fatalf("lost host, duplicated resume or unrelated binding=%+v", repo.items)
	}
	for _, item := range repo.items {
		raw, err := vault.Decrypt("xnode.endpoint."+item.ID, item.OverridesCipher)
		if err != nil {
			t.Fatal(err)
		}
		var options struct {
			Legacy struct {
				Policy struct {
					Mode   string   `json:"mode"`
					Squads []string `json:"squads"`
				} `json:"internalSquads"`
			} `json:"legacyHost"`
		}
		if err := json.Unmarshal([]byte(raw), &options); err != nil || options.Legacy.Policy.Mode != "ALLOW_ONLY" || len(options.Legacy.Policy.Squads) != 1 {
			t.Fatalf("host ACL discarded: %s %v", raw, err)
		}
	}
}

func TestPreviewReportsInvalidHostAccessPolicy(t *testing.T) {
	for _, policy := range []string{`{"mode":"UNKNOWN","squads":[]}`, `{"mode":"ALLOW_ONLY","squads":[]}`, `{"mode":"EXCLUDE","squads":"invalid"}`} {
		raw := json.RawMessage(`{"uuid":"host","internalSquads":` + policy + `}`)
		result := inspectSnapshot(Snapshot{Hosts: []json.RawMessage{raw}})
		if len(result.Issues) != 1 || result.Issues[0].Code != "INVALID_HOST" || !result.Issues[0].Blocking {
			t.Fatalf("unsafe host policy not reported: %+v", result)
		}
	}
}
