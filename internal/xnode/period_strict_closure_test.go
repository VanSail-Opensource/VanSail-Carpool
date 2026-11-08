package xnode_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/gorilla/websocket"
)

func heldClosureFixture(t *testing.T, f fixture) (*websocket.Conn, xnode.Identity, xnode.Period) {
	t.Helper()
	ctx := context.Background()
	identity, err := f.store.XNodeIdentityByUser(ctx, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	period, err := f.store.XNodePeriod(ctx, identity.CurrentPeriodID)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.store.TrackXNodePeriodNode(ctx, period.ID, f.node.ID); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(f.master)
	t.Cleanup(server.Close)
	_, token, err := f.service.Enrollment(ctx, f.node.ID)
	if err != nil {
		t.Fatal(err)
	}
	socket := agentSocket(t, server, token, fixtureAgent, 0)
	readFrame(t, socket, "state.sync")
	return socket, identity, period
}

func heldClosureCommand(t *testing.T, socket *websocket.Conn, expected string) wire.Message {
	t.Helper()
	message := readFrame(t, socket, "command")
	var request struct {
		Name string            `json:"name"`
		Args wire.TrafficFlush `json:"args"`
	}
	if err := wire.Decode(message.Data, &request); err != nil || request.Name != "traffic.flush" || request.Args.AccountingPeriod != expected || !request.Args.Quiesce {
		t.Fatalf("unexpected closure command: %+v, %v", request, err)
	}
	return message
}

func TestStrictClosureNetworkWaitDoesNotHoldGlobalMutationLock(t *testing.T) {
	for _, action := range []string{"quiesce", "refund", "reconcile", "accounting"} {
		t.Run(action, func(t *testing.T) {
			f := controlFixture(t, true)
			socket, identity, period := heldClosureFixture(t, f)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if action == "reconcile" {
				var err error
				period, err = f.store.BeginXNodePeriodClosure(ctx, period, time.Now().UTC())
				if err != nil {
					t.Fatal(err)
				}
			}
			done := make(chan error, 1)
			go func() {
				switch action {
				case "quiesce":
					done <- f.service.Quiesce(ctx, stringID(identity))
				case "refund":
					done <- f.service.QuiescePurchaseOperation(ctx, stringID(identity), period.PurchaseID)
				case "reconcile":
					_, err := f.service.PurchaseOperationState(ctx, stringID(identity), period.PurchaseID)
					done <- err
				case "accounting":
					purchases, err := f.store.ListPurchases(ctx, f.user.ID)
					if err == nil {
						err = f.service.CloseAccountingPeriod(ctx, stringID(identity), purchases[0].ValidFrom, purchases[0].ValidUntil)
					}
					done <- err
				}
			}()
			heldClosureCommand(t, socket, period.ID)
			current, err := f.store.XNodePeriod(ctx, period.ID)
			if err != nil || current.Status != "closing" || current.ClosureGeneration < 1 {
				t.Fatal("network preceded durable admission fence", current, err)
			}
			other, _, err := f.store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 99202, FirstName: "Other"}, false)
			if err != nil {
				t.Fatal(err)
			}
			independent := make(chan error, 1)
			go func() { _, err := f.service.EnsureIdentity(ctx, other); independent <- err }()
			select {
			case err = <-independent:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("agent barrier monopolized the global mutation lock")
			}
			cancel()
			select {
			case err = <-done:
				if err == nil {
					t.Fatal("unanswered barrier reported settlement success")
				}
			case <-time.After(3 * time.Second):
				t.Fatal("canceled closure did not stop")
			}
			participants, err := f.store.XNodePeriodNodes(context.Background(), period.ID)
			if err != nil || len(participants) != 1 || participants[0].Flushed {
				t.Fatal("unfinished accounting was sealed", participants, err)
			}
		})
	}
}

func completeHeldClosure(t *testing.T, socket *websocket.Conn, command wire.Message, period xnode.Period) {
	t.Helper()
	barrier := wire.TrafficBarrier{AgentID: fixtureAgent, AccountingPeriod: period.ID, Quiesced: true}
	if err := socket.WriteJSON(wire.New("command.result", command.ID, 0, map[string]any{"id": command.ID, "command": "traffic.flush", "status": "completed", "data": barrier})); err != nil {
		t.Fatal(err)
	}
}

func TestStrictClosureLateBarrierCannotCloseReopenedGeneration(t *testing.T) {
	f := controlFixture(t, true)
	socket, identity, period := heldClosureFixture(t, f)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- f.service.QuiescePurchaseOperation(ctx, stringID(identity), period.PurchaseID) }()
	command := heldClosureCommand(t, socket, period.ID)
	purchases, err := f.store.ListPurchases(ctx, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.service.ResumePurchase(ctx, stringID(identity), purchases[0]); err != nil {
		t.Fatal(err)
	}
	completeHeldClosure(t, socket, command, period)
	select {
	case err = <-done:
		if !errors.Is(err, xnode.ErrConflict) {
			t.Fatal("old closure adopted reopened admission", err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	current, err := f.store.XNodePeriod(ctx, period.ID)
	participants, participantsErr := f.store.XNodePeriodNodes(ctx, period.ID)
	if err != nil || participantsErr != nil || current.Status != "open" || len(participants) != 1 || participants[0].Flushed {
		t.Fatal("late barrier changed reopened accounting", current, participants, err, participantsErr)
	}
}

func TestStrictClosureLateBarrierPreservesConcurrentSuccessor(t *testing.T) {
	f := controlFixture(t, true)
	socket, identity, period := heldClosureFixture(t, f)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- f.service.QuiescePurchaseOperation(ctx, stringID(identity), period.PurchaseID) }()
	command := heldClosureCommand(t, socket, period.ID)
	next := period
	next.ID, next.Status, next.UsedBytes = "new-paid-allowance", "open", 77
	if err := f.store.SaveXNodePeriod(ctx, next); err != nil {
		t.Fatal(err)
	}
	// Usage is journal-owned; saving period policy deliberately cannot seed it.
	if _, err := f.store.DB().ExecContext(ctx, `UPDATE xnode_periods SET used_bytes=$1 WHERE id=$2`, next.UsedBytes, next.ID); err != nil {
		t.Fatal(err)
	}
	completeHeldClosure(t, socket, command, period)
	select {
	case err := <-done:
		if !errors.Is(err, xnode.ErrConflict) {
			t.Fatal("old refund reported quiescing its successor", err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	current, err := f.store.XNodePeriod(ctx, next.ID)
	owner, ownerErr := f.store.XNodeIdentityByUser(ctx, f.user.ID)
	if err != nil || ownerErr != nil || current.Status != "open" || current.UsedBytes != 77 || owner.CurrentPeriodID != next.ID {
		t.Fatalf("old closure changed successor: period=%s status=%s used=%d current_period=%s period_error=%v owner_error=%v", current.ID, current.Status, current.UsedBytes, owner.CurrentPeriodID, err, ownerErr)
	}
}
