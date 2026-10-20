package xnode_test

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	jobqueue "github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/routingmatch"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/gorilla/websocket"
	confserial "github.com/xtls/xray-core/infra/conf/serial"
	"net/http/httptest"
	"slices"
	"testing"
	"time"
)

type routingFrame struct {
	node, phase string
	transit     int
}

func routingAgent(ctx context.Context, f fixture, node, agent string, socket *websocket.Conn, hold string, release <-chan struct{}, frames chan<- routingFrame, blocked chan<- struct{}, errs chan<- error) {
	_ = socket.SetReadDeadline(time.Time{})
	send := func(m wire.Message) bool {
		if err := socket.WriteJSON(m); err != nil {
			select {
			case errs <- err:
			default:
			}
			return false
		}
		return true
	}
	for {
		var message wire.Message
		if err := socket.ReadJSON(&message); err != nil {
			return
		}
		switch message.Type {
		case "state.sync":
			var config wire.Config
			if err := wire.Decode(message.Data, &config); err != nil {
				errs <- err
				return
			}
			plan, err := f.store.XNodeRoutingPlan(ctx, f.identity.ProxyUserID)
			phase := "initial"
			if err == nil {
				phase = plan.Phase
			}
			count := 0
			for _, u := range config.Users {
				count += len(u.TransitCredentials)
			}
			select {
			case frames <- routingFrame{node, phase, count}:
			case <-ctx.Done():
				return
			}
			if node == hold && phase == "applying" {
				select {
				case blocked <- struct{}{}:
				default:
				}
				select {
				case <-release:
				case <-ctx.Done():
					return
				}
			}
			if !send(wire.New("state.accepted", "", message.Revision, map[string]any{})) || !send(wire.New("state.applied", "", message.Revision, map[string]any{})) {
				return
			}
			if !send(wire.New("telemetry.report", "", 0, wire.TelemetryReport{AgentID: agent, BootID: fixtureBoot, ConfigRevision: message.Revision, Time: time.Now(), Ready: true, Synchronized: true, Users: wire.UserMetrics{}})) {
				return
			}
		case "command":
			var request struct {
				Name string `json:"name"`
			}
			if json.Unmarshal(message.Data, &request) != nil {
				errs <- fmt.Errorf("invalid command")
				return
			}
			data := any(map[string]any{})
			if request.Name == "traffic.flush" {
				data = wire.TrafficBarrier{AgentID: agent}
			}
			if !send(wire.New("command.result", message.ID, 0, map[string]any{"id": message.ID, "command": request.Name, "status": "completed", "data": data, "finished": time.Now()})) {
				return
			}
		}
	}
}

func TestManagedRoutingNeverEnablesPeersBeforeEveryNodeApplies(t *testing.T) {
	f := controlFixture(t, true)
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	// The pinned core prohibits public plaintext VLESS peers. Use the same
	// encrypted server/client transport that a real managed route requires.
	key, keyErr := ecdh.X25519().GenerateKey(rand.Reader)
	if keyErr != nil {
		t.Fatal(keyErr)
	}
	profile, err := f.service.ResolveConfiguration(ctx, f.node.ConfigID)
	if err != nil {
		t.Fatal(err)
	}
	var native map[string]any
	if err = json.Unmarshal(profile.Document, &native); err != nil {
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
	other, err := f.service.SaveNode(ctx, xnode.Node{Name: "Peer", Enabled: true, Multiplier: 1, ConfigID: f.node.ConfigID}, xnode.NodeSettings{})
	if err != nil {
		t.Fatal(err)
	}
	f.group.Bindings = append(f.group.Bindings, xnode.GroupBinding{NodeID: other.ID, InboundTag: "paid"})
	if _, err = f.service.SaveGroup(ctx, f.group); err != nil {
		t.Fatal(err)
	}
	for index, node := range []xnode.Node{f.node, other} {
		if _, err = f.service.SaveEndpoint(ctx, xnode.Endpoint{Name: node.Name, NodeID: node.ID, InboundTag: "paid", Address: []string{"1.1.1.1", "8.8.8.8"}[index], Port: 443, Enabled: true}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = f.service.SaveRoutingTemplates(ctx, f.user.ID, xnode.RoutingTemplateConfig{Templates: []xnode.RoutingTemplate{{ID: "google", Name: "Google", Match: routingmatch.Match{Domains: []string{"domain:google.com"}}}, {ID: "unrelated", Name: "Unused GeoSite", Match: routingmatch.Match{Domains: []string{"geosite:youtube"}}}}}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(f.master)
	defer server.Close()
	ids := []string{f.node.ID, other.ID}
	slices.Sort(ids)
	// Only the source gains an outbound during applying. The destination is
	// deliberately unchanged until enabling, so it needs no redundant frame.
	hold := f.node.ID
	release := make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	frames := make(chan routingFrame, 64)
	blocked := make(chan struct{}, 1)
	errs := make(chan error, 4)
	for index, node := range []xnode.Node{f.node, other} {
		_, token, err := f.service.Enrollment(ctx, node.ID)
		if err != nil {
			t.Fatal(err)
		}
		agent := fmt.Sprintf("%032x", index+991)
		socket := agentSocket(t, server, token, agent, 0, "transit_credentials_v1")
		go routingAgent(ctx, f, node.ID, agent, socket, hold, release, frames, blocked, errs)
	}
	for _, id := range ids {
		if err = f.master.WaitApplied(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	receipt, err := f.service.QueueOwnedRouting(ctx, f.user.ID, "managed-graph", 0, []xnode.RoutingChoice{{ID: "first", InboundNodeID: f.node.ID, TemplateID: "google", OutboundNodeID: other.ID}})
	if err != nil {
		var diagnostic *xnode.CompilationError
		if errors.As(err, &diagnostic) {
			// A temporary, fixture-only probe lets the pinned parser explain a
			// rejected client projection without exposing production documents.
			endpoints, _ := f.service.Endpoints(ctx)
			if len(endpoints) > 0 {
				probe, probeErr := f.service.AuthorizeService(ctx, "route-diagnostic", endpoints[0], time.Now().Add(time.Minute))
				if probeErr == nil {
					client, clientErr := f.service.ServiceProxy(ctx, probe, endpoints[0])
					out, renderErr := client.Outbound()
					if renderErr == nil {
						raw, _ := json.Marshal(map[string]any{"inbounds": []any{}, "outbounds": []any{out}})
						_, coreErr := confserial.LoadJSONConfig(bytes.NewReader(raw))
						t.Logf("fixture native client parser: %v; projection: %v", coreErr, clientErr)
					}
				}
			}
			t.Fatalf("%v: %s", err, diagnostic.ValidationDetail())
		}
		t.Fatal(err)
	}
	var claimed bool
	for attempts := 0; attempts < 20; attempts++ {
		job, err := f.store.ClaimOutboxJob(ctx, time.Now().Add(time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		if job == nil {
			break
		}
		var payload struct {
			OperationID string `json:"operationId"`
		}
		_ = json.Unmarshal([]byte(job.Payload), &payload)
		if payload.OperationID == receipt.ID {
			operation, err := f.store.ProviderOperationByID(ctx, receipt.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err = f.service.HandleProviderOperation(ctx, operation, *job); !errors.Is(err, jobqueue.ErrDeferred) {
				t.Fatalf("routing wait was not deferred: %v", err)
			}
			claimed = true
			break
		}
		if err = f.store.CompleteOutboxJob(ctx, job.ID, job.Attempts, nil, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	if !claimed {
		t.Fatal("atomic routing outbox missing")
	}
	select {
	case <-blocked:
	case err := <-errs:
		t.Fatal(err)
	case <-ctx.Done():
		diagnosticCtx, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		plan, planErr := f.store.XNodeRoutingPlan(diagnosticCtx, f.identity.ProxyUserID)
		operation, opErr := f.store.ProviderOperationByID(diagnosticCtx, receipt.ID)
		jobs, _ := f.store.ListOutboxJobs(diagnosticCtx, 20)
		for _, job := range jobs {
			t.Logf("fixture job %s status %s error %s", job.Kind, job.Status, job.LastError)
		}
		t.Fatalf("apply phase did not reach held node: phase=%s plan=%v receipt=%s status=%s op=%v", plan.Phase, planErr, operation.Receipt.ID, operation.Receipt.Status, opErr)
	}
	plan, err := f.store.XNodeRoutingPlan(ctx, f.identity.ProxyUserID)
	if err != nil || plan.Phase != "applying" {
		t.Fatalf("crossed missing acknowledgement: %+v %v", plan, err)
	}
	for len(frames) > 0 {
		frame := <-frames
		if frame.transit != 0 {
			t.Fatalf("peer enabled before all applies: %+v", frame)
		}
	}
	close(release)
	for {
		operation, err := f.store.ProviderOperationByID(ctx, receipt.ID)
		if err != nil {
			t.Fatal(err)
		}
		if operation.Receipt.Status == "succeeded" {
			break
		}
		select {
		case err := <-errs:
			t.Fatal(err)
		case <-ctx.Done():
			t.Fatalf("routing did not finish: %+v", operation.Receipt)
		case <-time.After(50 * time.Millisecond):
		}
	}
	config, err := f.service.Compile(ctx, other.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Users) != 1 || len(config.Users[0].TransitCredentials) != 1 {
		t.Fatal("authenticated peer account not enabled")
	}
	period, err := f.store.XNodePeriod(ctx, config.Users[0].AccountingPeriod)
	if err != nil || period.Status != "open" || period.UsedBytes != 0 {
		t.Fatal("deployment changed accounting period", err)
	}
	// Revoking the purchased group removes route choices from durable operational
	// state and authenticated peers from the compiled node configuration.
	f.group.Enabled = false
	if _, err := f.service.SaveGroup(ctx, f.group); err != nil {
		t.Fatal(err)
	}
	for {
		plan, err := f.store.XNodeRoutingPlan(ctx, f.identity.ProxyUserID)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Reason == "routing_access_removed" {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("lost access retained routes")
		case <-time.After(50 * time.Millisecond):
		}
	}
	view, err := f.service.OwnedRoutingPlan(ctx, f.user.ID)
	if err != nil || len(view.Plan.Choices) != 0 {
		t.Fatal("database retained inaccessible choices", err)
	}
	revoked, err := f.service.Compile(ctx, other.ID)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(revoked.Xray, []byte("route-")) {
		t.Fatal("revoked route retained managed native rules or outbounds")
	}
	for _, user := range revoked.Users {
		if len(user.TransitCredentials) != 0 {
			t.Fatal("revoked route retained native peer credentials")
		}
	}

}
