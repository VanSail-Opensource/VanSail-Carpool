package xnode

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/hourly"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestHourlyFullOwnerCaptureAndACKBurstSurviveBlockedFinancialSQL(t *testing.T) {
	now := time.Now().UTC()
	base := masterHourlyRepo()
	base.progressEntered, base.trafficEntered = make(chan struct{}, 1), make(chan struct{}, 1)
	repo := &hourlyPolicySocketRepository{hourlySocketRepository: base}
	for i := range maxHourlyLiveOwners {
		owner := hourlyLaneDemand(i).Owner
		repo.policies = append(repo.policies, HourlyAccountPolicy{OwnerID: owner.ID, AccountGeneration: 1, PaidThrough: now.Add(time.Hour), RateBytesPerSecond: 1000, NodePaidThrough: map[string]time.Time{"node": now.Add(time.Hour)}})
	}
	m, server, token := masterHourlyFixture(t, repo)
	m.service.now = func() time.Time { return now }
	m.hourly.allocator = hourly.New(now.Add(-time.Minute), hourly.Limits{})
	m.refreshHourlyPolicies(context.Background())
	socket, _ := masterHourlySocket(t, server, token, masterHourlySession, wire.HourlyFormat, true)
	defer socket.Close()
	peer := waitMasterHourlyPeer(t, m)
	progress := wire.HourlyCreditProgress{ReservationID: "held-SQL", IssueSequence: 1, AdmittedUploadBytes: 1}
	if err := socket.WriteJSON(wire.New("hourly.demand", "", 0, wire.HourlyDemandBatch{SessionID: masterHourlySession, Progress: []wire.HourlyCreditProgress{progress}})); err != nil {
		t.Fatal(err)
	}
	select {
	case <-base.progressEntered:
	case <-time.After(time.Second):
		t.Fatal("financial SQL fixture did not block")
	}
	// One legal full capture is 64 demand frames plus 64 native-ACK frames.
	// The old 32-frame FIFO disconnected this exact peer while SQL was blocked.
	for start := 0; start < maxHourlyLiveOwners; start += wire.MaxHourlyItems {
		var demands []wire.HourlyDemand
		var acks []wire.HourlyGrantAck
		for i := start; i < start+wire.MaxHourlyItems; i++ {
			demand := hourlyLaneDemand(i)
			demands = append(demands, demand)
			acks = append(acks, wire.HourlyGrantAck{SessionID: masterHourlySession, Owner: demand.Owner, Nonce: demand.Nonce, Sequence: 1, Applied: true})
		}
		for _, frame := range []wire.Message{
			wire.New("hourly.demand", "", 0, wire.HourlyDemandBatch{SessionID: masterHourlySession, Demands: demands}),
			wire.New("hourly.grant.acks", "", 0, wire.HourlyGrantAckBatch{SessionID: masterHourlySession, Acks: acks}),
		} {
			if err := socket.WriteJSON(frame); err != nil {
				t.Fatal("legitimate owner burst disconnected", err)
			}
		}
	}
	pending := make(chan wire.Message, 1)
	peer.mu.Lock()
	peer.pending["independent-command"] = pending
	peer.mu.Unlock()
	if err := socket.WriteJSON(wire.New("command.result", "independent-command", 0, map[string]any{})); err != nil {
		t.Fatal(err)
	}
	select {
	case <-pending:
	case <-time.After(2 * time.Second):
		t.Fatal("financial wait/full owner capture blocked command results")
	}
	report := wire.TrafficReport{ID: masterHourlyAgent + ":1", Traffic: wire.Traffic{}}
	if err := socket.WriteJSON(wire.New("traffic.report", report.ID, 0, report)); err != nil {
		t.Fatal(err)
	}
	if err := socket.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var ack wire.Message
	for {
		if err := socket.ReadJSON(&ack); err != nil {
			t.Fatal("financial wait/full capture disconnected or blocked native ACK", err)
		}
		if ack.Type == "traffic.ack" {
			break
		}
		// A slow race runner can reach the intentional SQL timeout while
		// sending the full capture. Its error frame is independent of this ACK.
		if ack.Type != "error" {
			t.Fatal("unexpected frame while awaiting native receipt", ack.Type)
		}
	}
	var receipt struct {
		ID string `json:"id"`
	}
	if wire.Decode(ack.Data, &receipt) != nil || receipt.ID != report.ID || !m.currentHourlyPeer(peer) {
		t.Fatal("native receipt did not match the committed journal")
	}
	select {
	case <-base.trafficEntered:
	default:
		t.Fatal("native journal ACK bypassed its commit")
	}
}
