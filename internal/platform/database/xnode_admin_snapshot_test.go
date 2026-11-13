package database

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestNativeAdminReturnsCommittedRevisionsAndValidatedPreviews(t *testing.T) {
	t.Parallel()
	ctx, f := context.Background(), newXNodeAdminFixture(t)
	f.config.Name, f.config.Document = "renamed-config", json.RawMessage(`{"unpersisted":"document"}`)
	config, err := f.store.SaveAdminXNodeConfiguration(ctx, f.actor, f.config)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := f.store.XNodeConfiguration(ctx, config.ID)
	if err != nil || !reflect.DeepEqual(config, stored) || config.Revision != 2 || config.CreatedAt.IsZero() || len(config.Document) != 0 {
		t.Fatal("configuration did not return authoritative stored snapshot", err)
	}
	if _, err = f.store.SaveAdminXNodeConfiguration(ctx, f.actor, f.config); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("configuration revision guard was lost", err)
	}
	f.snippet.Name = "renamed-snippet"
	snippet, err := f.store.SaveAdminXNodeSnippet(ctx, f.actor, f.snippet)
	if err != nil || snippet.Revision != 2 || snippet.CreatedAt.IsZero() || snippet.Name != f.snippet.Name {
		t.Fatal("snippet did not return committed revision", err)
	}
	if _, err = f.store.SaveAdminXNodeSnippet(ctx, f.actor, f.snippet); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("snippet revision guard was lost", err)
	}
	f.group.Bindings[0].GroupID = "ignored-caller-parent"
	group, err := f.store.SaveAdminXNodeGroup(ctx, f.actor, f.group)
	if err != nil || len(group.Bindings) != 1 || group.Bindings[0].GroupID != group.ID || group.Name != f.group.Name {
		t.Fatal("group response did not reflect persisted ownership", group, err)
	}
	f.endpoint.Overrides = json.RawMessage(`{"remark":"validated"}`)
	f.endpoint.Metadata = &xnode.EndpointMetadata{Protocol: "vless", Transport: "tcp", Security: "tls"}
	endpoint, err := f.store.SaveAdminXNodeEndpoint(ctx, f.actor, f.endpoint)
	if err != nil || !reflect.DeepEqual(endpoint, f.endpoint) {
		t.Fatal("endpoint lost stored fields or validated preview", err)
	}
	f.node.Name = "renamed-node"
	node, err := f.store.SaveAdminXNodeNodeDDNS(ctx, f.actor, f.node, nil)
	if err != nil || node.Name != f.node.Name || node.CreatedAt.IsZero() || node.MultiplierFP != xnode.MultiplierScale {
		t.Fatal("node response was not stored snapshot", err)
	}
	f.custom.Outbounds, f.custom.Routing = json.RawMessage(`[]`), json.RawMessage(`{"rules":[]}`)
	custom, err := f.store.SaveAdminXNodeCustomization(ctx, f.actor, f.custom)
	if err != nil || custom.Revision != 2 || string(custom.Outbounds) != string(f.custom.Outbounds) || string(custom.Routing) != string(f.custom.Routing) {
		t.Fatal("customization lost committed revision or preview", err)
	}
	if _, err = f.store.SaveAdminXNodeCustomization(ctx, f.actor, f.custom); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("customization revision guard was lost", err)
	}
}

func TestNativeAdminResponseValidationPrecedesCommit(t *testing.T) {
	for _, target := range []struct{ table, column, id, attempt string }{
		{"xnode_configs", "id", "config", "configuration save"},
		{"xnode_snippets", "id", "snippet", "snippet save"},
		{"xnode_nodes", "id", "node", "node save"},
		{"xnode_identities", "service_name", "quota-service", "named service"},
	} {
		t.Run(target.table, func(t *testing.T) {
			t.Parallel()
			f := newXNodeAdminFixture(t)
			if _, err := f.store.DB().ExecContext(context.Background(), `UPDATE `+target.table+` SET created_at='invalid-retained-timestamp' WHERE `+target.column+`=$1`, target.id); err != nil {
				t.Fatal(err)
			}
			before := xnodeAdminSnapshot(t, f.store)
			for _, attempt := range f.attempts() {
				if attempt.name == target.attempt {
					if err := attempt.write(f.actor); err == nil {
						t.Fatal("invalid response retained a successful commit")
					}
				}
			}
			if after := xnodeAdminSnapshot(t, f.store); after != before {
				t.Fatal("response failure committed replacement state or audit")
			}
		})
	}
}
