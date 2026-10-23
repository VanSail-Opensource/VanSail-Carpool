package xnode_test

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestOnlineReplayBacklogDoesNotBlockPaidSuccessor(t *testing.T) {
	f := controlFixture(t, true)
	ctx := context.Background()
	identity, err := f.store.XNodeIdentityByUser(ctx, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.DB().ExecContext(ctx, `UPDATE xnode_periods SET used_bytes=12345 WHERE id=?`, identity.CurrentPeriodID); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(f.master)
	defer server.Close()
	_, token, err := f.service.Enrollment(ctx, f.node.ID)
	if err != nil {
		t.Fatal(err)
	}
	socket := agentSocket(t, server, token, fixtureAgent, 0)
	defer socket.Close()
	readFrame(t, socket, "state.sync")
	if err = f.store.TrackXNodePeriodNode(ctx, identity.CurrentPeriodID, f.node.ID); err != nil {
		t.Fatal(err)
	}
	// The connected agent deliberately answers no commands or journal barriers.
	call, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err = f.service.ResetTraffic(call, stringID(identity)); err != nil {
		t.Fatal("successor waited on an online journal", err)
	}
	after, err := f.store.XNodeIdentityByUser(ctx, f.user.ID)
	if err != nil || after.CurrentPeriodID == identity.CurrentPeriodID {
		t.Fatalf("successor missing: %+v, %v", after, err)
	}
	previous, err := f.store.XNodePeriod(ctx, identity.CurrentPeriodID)
	if err != nil || previous.Status != "closing" || previous.UsedBytes != 12345 {
		t.Fatalf("predecessor evidence changed: %+v, %v", previous, err)
	}
	participants, err := f.store.XNodePeriodNodes(ctx, identity.CurrentPeriodID)
	if err != nil || len(participants) != 1 || participants[0].Flushed {
		t.Fatalf("unacknowledged barrier was sealed: %+v, %v", participants, err)
	}
	if _, err = f.service.SaveEndpoint(ctx, xnode.Endpoint{NodeID: f.node.ID, InboundTag: "paid", Name: "Paid host", Address: "proxy.example", Port: 443, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	metadata, err := f.service.OwnedSubscription(ctx, f.user.ID)
	if err != nil || !metadata.Active || metadata.Incomplete || metadata.UsedBytes != "0" {
		t.Fatalf("successor export unavailable: %+v, %v", metadata, err)
	}
}
