package xnode_test

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	jobqueue "github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/routingmatch"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func ddnsRoutingFixture(t *testing.T) (fixture, xnode.Node, xnode.Endpoint, xnode.Endpoint) {
	t.Helper()
	f := controlFixture(t, true)
	ctx := context.Background()
	if _, err := f.store.DB().ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=?`, f.user.ID); err != nil {
		t.Fatal(err)
	}
	profile, err := f.service.ResolveConfiguration(ctx, f.node.ConfigID)
	if err != nil {
		t.Fatal(err)
	}
	var native map[string]any
	if err = json.Unmarshal(profile.Document, &native); err != nil {
		t.Fatal(err)
	}
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	inbound := native["inbounds"].([]any)[0].(map[string]any)
	inbound["streamSettings"] = map[string]any{"network": "tcp", "security": "reality", "realitySettings": map[string]any{"dest": "www.example.com:443", "serverNames": []string{"www.example.com"}, "privateKey": base64.RawURLEncoding.EncodeToString(key.Bytes()), "shortIds": []string{"0123456789abcdef"}}}
	profile.Document, err = json.Marshal(native)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.SaveConfiguration(ctx, profile); err != nil {
		t.Fatal(err)
	}
	peer, err := f.service.SaveNode(ctx, xnode.Node{Name: "DDNS peer", Enabled: true, Multiplier: 1, ConfigID: profile.ID}, xnode.NodeSettings{})
	if err != nil {
		t.Fatal(err)
	}
	f.group.Bindings = append(f.group.Bindings, xnode.GroupBinding{NodeID: peer.ID, InboundTag: "paid"})
	if _, err = f.service.SaveGroup(ctx, f.group); err != nil {
		t.Fatal(err)
	}
	hosts := []xnode.Endpoint{}
	for index, node := range []xnode.Node{f.node, peer} {
		host, err := f.service.SaveEndpoint(ctx, xnode.Endpoint{Name: node.Name, NodeID: node.ID, InboundTag: "paid", Address: []string{"1.1.1.1", "8.8.8.8"}[index], Port: 443, Enabled: true})
		if err != nil {
			t.Fatal(err)
		}
		hosts = append(hosts, host)
	}
	if _, err = f.service.SaveNodeAs(ctx, peer, xnode.NodeSettings{DDNS: []wire.DDNSJob{{ID: "wan", Provider: wire.BuiltinDDNSProvider, IPv4: "auto", Interface: "eth0"}}, DDNSBindings: map[string]xnode.DDNSHostBinding{"wan": {IPv4HostID: hosts[1].ID}}}, f.user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.SaveRoutingTemplates(ctx, f.user.ID, xnode.RoutingTemplateConfig{Templates: []xnode.RoutingTemplate{{ID: "search", Name: "Search", Match: routingmatch.Match{Domains: []string{"domain:example.com"}}}}}); err != nil {
		t.Fatal(err)
	}
	return f, peer, hosts[0], hosts[1]
}

// Drain the real durable handlers under the fixture's advanced delivery clock.
// Bootstrap purchase jobs were already applied by controlFixture; this fixture
// exercises only DDNS, routing and protected-bank effects.
func pumpDDNSJobs(t *testing.T, ctx context.Context, f fixture) {
	t.Helper()
	for index := 0; index < 12; index++ {
		job, err := f.store.ClaimOutboxJob(ctx, time.Now().Add(time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		if job == nil {
			return
		}
		var callErr error
		if job.Kind == "provider_operation" {
			var payload struct {
				OperationID string `json:"operationId"`
			}
			if err = json.Unmarshal([]byte(job.Payload), &payload); err != nil {
				t.Fatal(err)
			}
			op, err := f.store.ProviderOperationByID(ctx, payload.OperationID)
			if err != nil {
				t.Fatal(err)
			}
			if op.Receipt.Kind == xnode.RoutingOperationKind || op.Receipt.Kind == xnode.DDNSOperationKind {
				callErr = f.service.HandleProviderOperation(ctx, op, *job)
			}
		} else if job.Kind == xnode.FirewallJob {
			callErr = f.master.HandleOutbox(ctx, *job)
		}
		if !errors.Is(callErr, jobqueue.ErrDeferred) {
			if err = f.store.CompleteOutboxJob(ctx, job.ID, job.Attempts, callErr, time.Now()); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func waitDDNS(t *testing.T, ctx context.Context, f fixture, description string, predicate func() bool) {
	t.Helper()
	for {
		pumpDDNSJobs(t, ctx, f)
		if predicate() {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal(description, ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
}
