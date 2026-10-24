package xnode_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/gorilla/websocket"
)

func TestFundedOnboardingSyncsPAYGThroughOutboxAndAgentBarrier(t *testing.T) {
	f := controlFixture(t, false)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := f.store.SaveSquadProduct(ctx, database.SquadProductInput{GroupID: f.group.ID, UpstreamPresent: true, BillingMode: "payg", PAYGPriceTXBMinor: 100}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.SaveEndpoint(ctx, xnode.Endpoint{NodeID: f.node.ID, InboundTag: "paid", Name: "PAYG host", Address: "proxy.example", Port: 443, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	user, _, err := f.store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 99103, FirstName: "PAYG member"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.AdjustBalance(ctx, user.ID, 100, "fund-before-onboarding", "fund", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := f.store.BeginAgreement(ctx, user.ID); err != nil {
		t.Fatal(err)
	}
	revision, ids, err := f.store.CurrentAgreementContract(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.CompleteOnboardingRevision(ctx, user.ID, revision, ids, time.Now()); err != nil {
		t.Fatal(err)
	}
	worker := outbox.NewWorker(f.store)
	for _, kind := range []string{xnode.AccessRepairJob, xnode.PAYGJob, xnode.PAYGRecoveryJob} {
		if err := worker.Register(kind, f.master); err != nil {
			t.Fatal(err)
		}
	}
	server := httptest.NewServer(f.master)
	defer server.Close()
	_, token, err := f.service.Enrollment(ctx, f.node.ID)
	if err != nil {
		t.Fatal(err)
	}
	socket := agentSocket(t, server, token, fixtureAgent, 0)
	defer socket.Close()
	states, failures := make(chan []wire.User, 32), make(chan error, 1)
	done := make(chan struct{})
	go func() { defer close(done); onboardingPAYGAgent(ctx, socket, states, failures) }()
	t.Cleanup(func() { cancel(); _ = socket.Close(); <-done })
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			t.Fatal("funded member never received PAYG authorization", ctx.Err())
		case err := <-failures:
			t.Fatal(err)
		case <-tick.C:
			if err := worker.Drain(ctx, 20); err != nil {
				t.Fatal(err)
			}
		case users := <-states:
			accounts, err := f.store.XNodePAYGAccounts(ctx, user.ID, f.node.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(accounts) != 1 {
				continue
			}
			a := accounts[0]
			for _, admitted := range users {
				if int64(admitted.ID) != a.ProxyUserID {
					continue
				}
				if admitted.AccountingPeriod != a.PeriodID || admitted.ValidUntil == nil || !admitted.ValidUntil.After(time.Now()) {
					t.Fatal("PAYG authorization lost its period or lease", admitted)
				}
				if a.BalanceMinor != 100 || a.PurchasedBytes != 0 || a.UsedBytes != 0 {
					t.Fatal("onboarding charged idle PAYG", a)
				}
				metadata, err := f.service.OwnedSubscription(ctx, user.ID)
				if err != nil || !metadata.Active || metadata.URL == "" {
					t.Fatal("PAYG subscription unavailable", metadata, err)
				}
				return
			}
		}
	}
}

func onboardingPAYGAgent(ctx context.Context, socket *websocket.Conn, states chan<- []wire.User, failures chan<- error) {
	_ = socket.SetReadDeadline(time.Time{})
	send := func(message wire.Message) bool { return socket.WriteJSON(message) == nil }
	for {
		var message wire.Message
		if socket.ReadJSON(&message) != nil {
			return
		}
		switch message.Type {
		case "state.sync":
			var config wire.Config
			if err := wire.Decode(message.Data, &config); err != nil {
				failures <- err
				return
			}
			if !send(wire.New("state.accepted", "", message.Revision, map[string]any{})) || !send(wire.New("state.applied", "", message.Revision, map[string]any{})) {
				return
			}
			report := wire.TelemetryReport{AgentID: fixtureAgent, BootID: fixtureBoot, ConfigRevision: message.Revision, Time: time.Now(), Ready: true, Synchronized: true, Users: wire.UserMetrics{}}
			if !send(wire.New("telemetry.report", "", 0, report)) {
				return
			}
			select {
			case states <- config.Users:
			case <-ctx.Done():
				return
			}
		case "command":
			var request struct {
				Name string `json:"name"`
			}
			if json.Unmarshal(message.Data, &request) != nil || request.Name != "traffic.flush" {
				failures <- fmt.Errorf("unexpected PAYG recovery command: %s", request.Name)
				return
			}
			barrier := wire.TrafficBarrier{AgentID: fixtureAgent, Through: time.Now()}
			if !send(wire.New("command.result", message.ID, 0, map[string]any{"id": message.ID, "command": request.Name, "status": "completed", "data": barrier, "finished": time.Now()})) {
				return
			}
		}
	}
}
