package xnode

import (
	"context"
	"fmt"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func hourlyLaneProof(i int, recovery bool) wire.HourlyCreditProof {
	install := wire.HourlyCreditInstall{ReservationID: fmt.Sprintf("credit-%05d", i), IssueSequence: uint64(i + 1), Owner: wire.HourlyOwner{ID: "owner", Generation: 1}, DayUTC: "2026-11-09", AgentID: masterHourlyAgent, BootID: masterHourlyBoot, SessionID: masterHourlySession, ByteCeiling: 100, Kind: "free"}
	proof := wire.HourlyCreditProof{Install: install, UnusedBytes: 100}
	if recovery {
		proof.UnusedBytes, proof.UnknownBytes, proof.Recovery = 0, 100, true
	}
	proof.Digest = wire.HourlyProofDigest(proof)
	return proof
}

func TestHourlyFinancialRecoveryRetainsProgressAcrossCancelledSQL(t *testing.T) {
	q := newHourlyFinancialLane()
	at := time.Now()
	proof := hourlyLaneProof(0, true)
	progress := wire.HourlyCreditProgress{ReservationID: proof.Install.ReservationID, IssueSequence: proof.Install.IssueSequence, AdmittedUploadBytes: 10, AdmittedDownloadBytes: 20}
	if q.offerProgress([]wire.HourlyCreditProgress{progress}) != nil || q.offerProofs([]wire.HourlyCreditProof{proof}) != nil {
		t.Fatal("valid financial evidence rejected")
	}
	work := q.take(at)
	if len(work) != 1 || work[0].progress == nil || work[0].proof == nil {
		t.Fatal("recovery zero counters erased pending admitted progress")
	}
	progress.AdmittedUploadBytes = 30
	if q.offerProgress([]wire.HourlyCreditProgress{progress}) != nil {
		t.Fatal("in-flight cumulative update rejected")
	}
	q.finish(work, context.DeadlineExceeded, map[string]error{progress.ReservationID: context.DeadlineExceeded}, at)
	if len(q.take(at.Add(time.Second-time.Nanosecond))) != 0 {
		t.Fatal("failed SQL retried before the bounded retry deadline")
	}
	work = q.take(at.Add(time.Second))
	if len(work) != 1 || work[0].proof.Digest != proof.Digest || *work[0].progress != progress {
		t.Fatal("SQL cancellation lost exact replayable proof or newer progress")
	}
	// A successful recovery proof never makes failed progress redundant.
	q.finish(work, context.Canceled, map[string]error{}, at.Add(time.Second))
	work = q.take(at.Add(2 * time.Second))
	if len(work) != 1 || work[0].proof != nil || work[0].progress == nil || *work[0].progress != progress {
		t.Fatal("unknown recovery discarded the still-uncommitted progress")
	}
	q.finish(work, nil, nil, at.Add(2*time.Second))
	if len(q.items) != 0 || q.bytes != 0 {
		t.Fatal("durably accepted copies retained unbounded transport state")
	}
}

func TestHourlyFinancialCapacityDropsWithoutACKAndAcceptsExactReplay(t *testing.T) {
	q := newHourlyFinancialLane()
	at := time.Now()
	for start := 0; start < maxHourlyFinancialIDs; start += wire.MaxHourlyItems {
		var batch []wire.HourlyCreditProof
		for i := start; i < start+wire.MaxHourlyItems; i++ {
			batch = append(batch, hourlyLaneProof(i, false))
		}
		if err := q.offerProofs(batch); err != nil {
			t.Fatal(err)
		}
	}
	extra := hourlyLaneProof(maxHourlyFinancialIDs, false)
	if err := q.offerProofs([]wire.HourlyCreditProof{extra}); err != nil || len(q.items) != maxHourlyFinancialIDs || q.items[extra.Install.ReservationID] != nil || q.dropped.Load() != 1 || q.bytes > maxHourlyFinancialBytes {
		t.Fatal("bounded financial overload disconnected or evicted evidence", err)
	}
	work := q.take(at)
	q.finish(work, nil, map[string]error{}, at)
	if err := q.offerProofs([]wire.HourlyCreditProof{extra}); err != nil || q.items[extra.Install.ReservationID] == nil || *q.items[extra.Install.ReservationID].proof != extra {
		t.Fatal("unacknowledged overflow could not replay after capacity returned", err)
	}
}

func TestHourlyFinancialRejectsConflictingIssueAndFinalDigest(t *testing.T) {
	q := newHourlyFinancialLane()
	proof := hourlyLaneProof(0, true)
	if q.offerProofs([]wire.HourlyCreditProof{proof}) != nil {
		t.Fatal("proof rejected")
	}
	progress := wire.HourlyCreditProgress{ReservationID: proof.Install.ReservationID, IssueSequence: proof.Install.IssueSequence + 1}
	if q.offerProgress([]wire.HourlyCreditProgress{progress}) == nil {
		t.Fatal("different issue merged with immutable proof")
	}
	changed := proof
	changed.Install.ByteCeiling++
	changed.UnknownBytes++
	changed.Digest = wire.HourlyProofDigest(changed)
	if q.offerProofs([]wire.HourlyCreditProof{changed}) == nil || *q.items[proof.Install.ReservationID].proof != proof {
		t.Fatal("different final digest/scope replaced immutable evidence")
	}
}
