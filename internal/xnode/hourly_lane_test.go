package xnode

import (
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestHourlyBlockedCreditProgressCannotDelayCommandsOrTrafficACK(t *testing.T) {
	repo := masterHourlyRepo()
	repo.progressEntered, repo.trafficEntered = make(chan struct{}, 1), make(chan struct{}, 1)
	m, server, token := masterHourlyFixture(t, repo)
	socket, _ := masterHourlySocket(t, server, token, masterHourlySession, wire.HourlyFormat, true)
	defer socket.Close()
	peer := waitMasterHourlyPeer(t, m)
	pending := make(chan wire.Message, 1)
	peer.mu.Lock()
	peer.pending["independent-command"] = pending
	peer.mu.Unlock()
	progress := wire.HourlyCreditProgress{ReservationID: "closed-owner-reservation", IssueSequence: 1, AdmittedUploadBytes: 1}
	batch := wire.HourlyDemandBatch{SessionID: masterHourlySession, Demands: []wire.HourlyDemand{}, Progress: []wire.HourlyCreditProgress{progress}}
	if err := socket.WriteJSON(wire.New("hourly.demand", "", 0, batch)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-repo.progressEntered:
	case <-time.After(time.Second):
		t.Fatal("closed-owner progress was discarded instead of reaching settlement")
	}
	if err := socket.WriteJSON(wire.New("command.result", "independent-command", 0, map[string]any{})); err != nil {
		t.Fatal(err)
	}
	select {
	case <-pending:
	case <-time.After(time.Second):
		t.Fatal("hourly database work starved command result delivery")
	}
	report := wire.TrafficReport{ID: masterHourlyAgent + ":1", Traffic: wire.Traffic{}}
	if err := socket.WriteJSON(wire.New("traffic.report", report.ID, 0, report)); err != nil {
		t.Fatal(err)
	}
	if err := socket.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var ack wire.Message
	if err := socket.ReadJSON(&ack); err != nil || ack.Type != "traffic.ack" {
		t.Fatal("hourly database work delayed traffic acknowledgment", err)
	}
	select {
	case <-repo.trafficEntered:
	default:
		t.Fatal("traffic ACK bypassed accounting commit")
	}
}

func TestHourlyInboundLanesValidateBeforeCoalescingAndClearBounds(t *testing.T) {
	m, peer, _, now := masterHourlyRateFixture(t)
	r := &peerReceivers{master: m, peer: peer, hourly: newHourlyInboundLane()}
	demand := wire.HourlyDemand{Owner: wire.HourlyOwner{ID: "owner", Generation: 1}, Nonce: masterHourlySession, LifetimeMillis: 10000, PendingBytes: 1, MinAtomicBytes: 1}
	message := wire.New("hourly.demand", "", 0, wire.HourlyDemandBatch{SessionID: peer.hourlySession, Demands: []wire.HourlyDemand{demand}})
	for range 64 {
		if err := r.offerHourly(message); err != nil {
			t.Fatal(err)
		}
	}
	if q := r.hourly.live; q.n != 1 || len(q.items) != 1 || q.bytes > maxHourlyLiveBytes || !q.items[demand.Owner].demand.receivedAt.Equal(*now) {
		t.Fatal("duplicate snapshots grew state or refreshed the nonce receipt")
	}
	for _, invalid := range []wire.Message{
		{Type: "hourly.demand", Data: make([]byte, wire.MaxHourlyFrameBytes+1)},
		wire.New("hourly.demand", "", 0, wire.HourlyDemandBatch{SessionID: "wrong", Demands: []wire.HourlyDemand{demand}}),
		wire.New("hourly.grant.acks", "", 0, wire.HourlyGrantAckBatch{SessionID: peer.hourlySession}),
	} {
		if err := r.offerHourly(invalid); err == nil || r.hourly.live.n != 1 {
			t.Fatal("invalid input changed pending controls", err)
		}
	}
	r.drainHourly()
	if r.hourly.live.n != 0 || r.hourly.live.bytes != 0 || len(r.hourly.financial.items) != 0 || r.hourly.financial.bytes != 0 {
		t.Fatal("hourly cleanup retained pending transport state")
	}
}
