package xnode

import (
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestHourlyDeliveredCreditDoesNotPollLedgerEverySchedulerTick(t *testing.T) {
	m, peer, r, now := hourlyPollingFixture(t, 1)
	runHourlyPollingPass(t, m, peer, *now)
	start := *now
	for n := 1; n < 10; n++ {
		*now = start.Add(time.Duration(n) * 100 * time.Millisecond)
		if peers := m.hourlyCredits.ready(*now); len(peers) != 0 {
			t.Fatal("unchanged delivered credit or fresh activity restarted SQL", n)
		}
	}
	if r.scans != 1 || r.held != 1 || r.days != 0 {
		t.Fatal("quiet credit repeated authoritative reads", r.scans, r.held, r.days)
	}
	*now = start.Add(hourlyCreditPollInterval)
	runHourlyPollingPass(t, m, peer, *now)
	if r.scans != 2 || r.held != 2 || r.days != 0 {
		t.Fatal("periodic reconciliation missed its exact service-clock boundary", r.scans, r.held, r.days)
	}
}

func TestHourlyCreditEventsBypassQuietPollingDeadline(t *testing.T) {
	for _, event := range []string{"positive-demand", "confirmed-progress", "proof-ack", "retirement", "resume"} {
		t.Run(event, func(t *testing.T) {
			m, peer, r, now := hourlyPollingFixture(t, 1)
			runHourlyPollingPass(t, m, peer, *now)
			*now = now.Add(100 * time.Millisecond)
			owner := wire.HourlyOwner{ID: "owner", Generation: 1}
			switch event {
			case "positive-demand":
				m.requestHourlyCredit(peer, []wire.HourlyDemand{{Owner: owner, Nonce: masterHourlySession, LifetimeMillis: 10000, PendingBytes: 1, MinAtomicBytes: 1}})
			case "confirmed-progress":
				row := r.rows[0]
				row.AdmittedUploadBytes = 1
				m.observeHourlyCreditProgress(peer, []model.HourlyCreditReservation{row})
				m.requestHourlyCredit(peer, nil)
			case "proof-ack":
				if err := m.queueHourlyCreditAck(peer, wire.HourlyCreditAck{ReservationID: "settled", IssueSequence: 2, Digest: "proof"}); err != nil {
					t.Fatal(err)
				}
			case "retirement":
				m.retireHourlyCreditScope(owner.ID, int64(owner.Generation), "")
			case "resume":
				if err := m.ResumeHourlyCreditAdmission(owner.ID, int64(owner.Generation)); err != nil {
					t.Fatal(err)
				}
			}
			if peers := m.hourlyCredits.ready(*now); len(peers) != 1 || peers[0] != peer {
				t.Fatal("new work waited for routine polling", event)
			}
			m.hourlyCredits.finished(peer, false, false, *now)
		})
	}
}

func TestHourlyZeroOrDuplicateActivityDoesNotRearmQuietLedgerReads(t *testing.T) {
	m, peer, r, now := hourlyPollingFixture(t, 1)
	runHourlyPollingPass(t, m, peer, *now)
	*now = now.Add(100 * time.Millisecond)
	m.requestHourlyCredit(peer, []wire.HourlyDemand{{Owner: wire.HourlyOwner{ID: "owner", Generation: 1}, Nonce: masterHourlySession, LifetimeMillis: 10000}})
	m.observeHourlyCreditProgress(peer, r.rows)
	m.requestHourlyCredit(peer, nil)
	if peers := m.hourlyCredits.ready(*now); len(peers) != 0 {
		t.Fatal("zero-pending keepalive or unchanged counters rearmed polling")
	}
}

func TestHourlyQuietScanPreservesConcurrentWorkAndFailureRetry(t *testing.T) {
	m, peer, r, now := hourlyPollingFixture(t, 1)
	r.onScan = func() {
		if err := m.ResumeHourlyCreditAdmission("owner", 1); err != nil {
			t.Fatal(err)
		}
	}
	runHourlyPollingPass(t, m, peer, *now)
	if peers := m.hourlyCredits.ready(*now); len(peers) != 1 {
		t.Fatal("completed scan erased work arriving during SQL")
	}
	m.hourlyCredits.finished(peer, false, true, *now)
	if peers := m.hourlyCredits.ready(now.Add(time.Second - time.Nanosecond)); len(peers) != 0 {
		t.Fatal("new work bypassed failure retry backoff")
	}
	if peers := m.hourlyCredits.ready(now.Add(time.Second)); len(peers) != 1 {
		t.Fatal("failed work disappeared behind quiet scheduling")
	}
	m.hourlyCredits.finished(peer, false, false, now.Add(time.Second))
}

func TestHourlyNewOwnersDrainWithoutPerPassPollingDelay(t *testing.T) {
	m, peer, r, now := hourlyPollingFixture(t, 9)
	for _, expected := range []int{4, 8, 9} {
		runHourlyPollingPass(t, m, peer, *now)
		if r.held != expected || r.days != 0 {
			t.Fatal("fair owner budget revisited checked owners or delayed new demand", r.held, r.days, expected)
		}
	}
	if peers := m.hourlyCredits.ready(*now); len(peers) != 0 {
		t.Fatal("drained owner demand remained a busy polling loop")
	}
}

func TestHourlyAmpleHeldCreditSkipsDayLocksButLowWaterRemainsFresh(t *testing.T) {
	for _, tc := range []struct {
		name string
		held int64
		days int
	}{
		{"above-half", hourlyCreditQuantum/2 + 1, 0},
		{"exact-half", hourlyCreditQuantum / 2, 1},
		{"below-half", hourlyCreditQuantum/2 - 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, peer, r, now := hourlyPollingFixture(t, 1)
			r.heldBytes, r.emptyDay = tc.held, true
			runHourlyPollingPass(t, m, peer, *now)
			if r.held != 1 || r.days != tc.days {
				t.Fatal("held-credit shortcut crossed the exact low-water boundary", r.held, r.days)
			}
		})
	}
}

func TestHourlyReservationPagesDrainBeforeQuietPolling(t *testing.T) {
	m, peer, r, now := hourlyPollingFixture(t, 0)
	state := m.hourlyCredits.peers[peer]
	for n := 1; n <= wire.MaxHourlyItems; n++ {
		row := r.rows[0]
		row.ReservationID = fmt.Sprintf("delivered-%d", n)
		row.IssueSequence = int64(n + 1)
		r.rows = append(r.rows, row)
		state.delivered[row.ReservationID] = row.IssueSequence
	}
	runHourlyPollingPass(t, m, peer, *now)
	runHourlyPollingPass(t, m, peer, *now)
	if r.scans != 2 || !state.scanned || state.scanIssue != 0 {
		t.Fatal("quiet cadence interrupted bounded obligation pagination")
	}
	if peers := m.hourlyCredits.ready(*now); len(peers) != 0 {
		t.Fatal("completed delivered pages resumed busy polling")
	}
}
