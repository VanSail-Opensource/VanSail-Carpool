package xnode_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestDiscardedTrafficIsAcknowledgedAndReplayIsIdempotent(t *testing.T) {
	f := controlFixture(t, false)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server := httptest.NewServer(f.master)
	defer server.Close()
	_, token, err := f.service.Enrollment(ctx, f.node.ID)
	if err != nil {
		t.Fatal(err)
	}
	socket := agentSocket(t, server, token, fixtureAgent, 0)
	at := time.Now().UTC()
	report := wire.TrafficReport{ID: fixtureAgent + ":1", Samples: []wire.TrafficSample{{UserID: 123, AccountingPeriod: "missing-period", From: at, Through: at, UploadBytes: 7}}}
	for range 2 {
		if err = socket.WriteJSON(wire.New("traffic.report", report.ID, 0, report)); err != nil {
			t.Fatal(err)
		}
		ack := readFrame(t, socket, "traffic.ack")
		var payload struct {
			ID string `json:"id"`
		}
		if err = wire.Decode(ack.Data, &payload); err != nil || payload.ID != report.ID {
			t.Fatalf("discard acknowledgement: %+v, %v", payload, err)
		}
	}
	var audits int
	var detail string
	if err = f.store.DB().QueryRowContext(ctx, `SELECT COUNT(*),MAX(detail) FROM audit_events WHERE action='xnode_traffic_discard'`).Scan(&audits, &detail); err != nil || audits != 1 {
		t.Fatalf("discard replay audit: %d, %v", audits, err)
	}
	var evidence struct {
		MissingPeriod string `json:"missingPeriod"`
	}
	if err = json.Unmarshal([]byte(detail), &evidence); err != nil || evidence.MissingPeriod != "missing-period" {
		t.Fatalf("discard diagnostic: %+v, %v", evidence, err)
	}
	sequence, err := f.store.XNodeReportSequence(ctx, fixtureAgent)
	if err != nil || sequence != 1 {
		t.Fatalf("discard cursor: %d, %v", sequence, err)
	}
	second := wire.TrafficReport{ID: fixtureAgent + ":2", Traffic: wire.Traffic{}}
	if err = socket.WriteJSON(wire.New("traffic.report", second.ID, 0, second)); err != nil {
		t.Fatal(err)
	}
	readFrame(t, socket, "traffic.ack")
	sequence, err = f.store.XNodeReportSequence(ctx, fixtureAgent)
	if err != nil || sequence != 2 {
		t.Fatalf("next report cursor: %d, %v", sequence, err)
	}
}
