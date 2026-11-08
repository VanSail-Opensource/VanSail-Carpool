package xnode

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

const maxHourlyFinancialIDs = 1024
const maxHourlyFinancialBytes = 2 << 20

type hourlyFinancialEntry struct {
	progress           *wire.HourlyCreditProgress
	proof              *wire.HourlyCreditProof
	proofBytes         int
	queued, processing bool
	retryAt            time.Time
}

type hourlyFinancialLane struct {
	mu         sync.Mutex
	items      map[string]*hourlyFinancialEntry
	order      [maxHourlyFinancialIDs]string
	head, n    int
	bytes      int
	batchLimit int
	wake       chan struct{}
	dropped    atomic.Uint64 // No ACK was issued; the agent must replay evidence.
}

func newHourlyFinancialLane() *hourlyFinancialLane {
	return &hourlyFinancialLane{items: map[string]*hourlyFinancialEntry{}, batchLimit: wire.MaxHourlyItems, wake: make(chan struct{}, 1)}
}

func hourlyFinancialSize(e *hourlyFinancialEntry) int {
	if e == nil {
		return 0
	}
	size := e.proofBytes
	if e.progress != nil {
		size += len(e.progress.ReservationID) + 256
	}
	return size
}

func (q *hourlyFinancialLane) put(id string, next *hourlyFinancialEntry) {
	previous := q.items[id]
	size := hourlyFinancialSize(next)
	if previous == nil && len(q.items) >= maxHourlyFinancialIDs || q.bytes-hourlyFinancialSize(previous)+size > maxHourlyFinancialBytes {
		q.dropped.Add(1)
		return
	}
	if !next.queued && !next.processing {
		q.order[(q.head+q.n)%maxHourlyFinancialIDs] = id
		q.n++
		next.queued = true
	}
	q.bytes += size - hourlyFinancialSize(previous)
	q.items[id] = next
}

func hourlyFinancialCompatible(e *hourlyFinancialEntry) bool {
	if e.progress == nil || e.proof == nil {
		return true
	}
	p, proof := e.progress, e.proof
	if p.IssueSequence != proof.Install.IssueSequence || p.AdmittedUploadBytes > proof.Install.ByteCeiling || p.AdmittedDownloadBytes > proof.Install.ByteCeiling-p.AdmittedUploadBytes {
		return false
	}
	return proof.Recovery || p.AdmittedUploadBytes <= proof.AdmittedUploadBytes && p.AdmittedDownloadBytes <= proof.AdmittedDownloadBytes
}

func (q *hourlyFinancialLane) offerProgress(items []wire.HourlyCreditProgress) error {
	if len(items) > wire.MaxHourlyItems {
		return ErrInvalid
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	updates := map[string]*hourlyFinancialEntry{}
	for _, progress := range items {
		if wire.ValidateHourlyProgress(progress) != nil {
			return ErrInvalid
		}
		next := hourlyFinancialEntry{}
		if previous := q.items[progress.ReservationID]; previous != nil {
			next = *previous
		}
		if previous := next.progress; previous != nil {
			if previous.IssueSequence != progress.IssueSequence {
				return ErrInvalid
			}
			progress.AdmittedUploadBytes = max(progress.AdmittedUploadBytes, previous.AdmittedUploadBytes)
			progress.AdmittedDownloadBytes = max(progress.AdmittedDownloadBytes, previous.AdmittedDownloadBytes)
		}
		next.progress = &progress
		if wire.ValidateHourlyProgress(progress) != nil || !hourlyFinancialCompatible(&next) {
			return ErrInvalid
		}
		updates[progress.ReservationID] = &next
	}
	for id, next := range updates {
		q.put(id, next)
	}
	if len(items) > 0 {
		hourlyLaneSignal(q.wake)
	}
	return nil
}

func (q *hourlyFinancialLane) offerProofs(items []wire.HourlyCreditProof) error {
	if len(items) > wire.MaxHourlyItems {
		return ErrInvalid
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	updates := map[string]*hourlyFinancialEntry{}
	for _, proof := range items {
		if wire.ValidateHourlyProof(proof) != nil {
			return ErrInvalid
		}
		next := hourlyFinancialEntry{}
		if previous := q.items[proof.Install.ReservationID]; previous != nil {
			next = *previous
		}
		if next.proof != nil && *next.proof != proof {
			return ErrInvalid
		}
		encoded, _ := json.Marshal(proof)
		next.proof, next.proofBytes = &proof, len(encoded)+128
		if !hourlyFinancialCompatible(&next) {
			return ErrInvalid
		}
		updates[proof.Install.ReservationID] = &next
	}
	for id, next := range updates {
		q.put(id, next)
	}
	if len(items) > 0 {
		hourlyLaneSignal(q.wake)
	}
	return nil
}

func (q *hourlyFinancialLane) clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = map[string]*hourlyFinancialEntry{}
	q.order, q.head, q.n, q.bytes = [maxHourlyFinancialIDs]string{}, 0, 0, 0
}
