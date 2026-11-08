package xnode

import (
	"context"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type hourlyFinancialWork struct {
	id       string
	progress *wire.HourlyCreditProgress
	proof    *wire.HourlyCreditProof
}

func (q *hourlyFinancialLane) take(now time.Time) []hourlyFinancialWork {
	q.mu.Lock()
	defer q.mu.Unlock()
	var result []hourlyFinancialWork
	for range q.n {
		if len(result) >= q.batchLimit {
			break
		}
		id := q.order[q.head]
		q.order[q.head] = ""
		q.head, q.n = (q.head+1)%maxHourlyFinancialIDs, q.n-1
		entry := q.items[id]
		if entry.retryAt.After(now) {
			q.order[(q.head+q.n)%maxHourlyFinancialIDs] = id
			q.n++
			continue
		}
		entry.queued, entry.processing = false, true
		result = append(result, hourlyFinancialWork{id, entry.progress, entry.proof})
	}
	return result
}

func (q *hourlyFinancialLane) finish(work []hourlyFinancialWork, progressErr error, proofErrors map[string]error, now time.Time) {
	q.mu.Lock()
	defer q.mu.Unlock()
	progressCount := 0
	for _, item := range work {
		previous := q.items[item.id]
		next := *previous
		failed := false
		if item.progress != nil {
			progressCount++
			if progressErr == nil && next.progress != nil && *next.progress == *item.progress {
				next.progress = nil
			}
			failed = progressErr != nil
		}
		if item.proof != nil {
			if proofErrors[item.id] == nil && next.proof != nil && *next.proof == *item.proof {
				next.proof, next.proofBytes = nil, 0
			}
			failed = failed || proofErrors[item.id] != nil
		}
		if next.progress == nil && next.proof == nil {
			q.bytes -= hourlyFinancialSize(previous)
			delete(q.items, item.id)
			continue
		}
		next.processing, next.retryAt = false, now
		if failed {
			next.retryAt = now.Add(time.Second)
		}
		q.put(item.id, &next)
	}
	// A single invalid/unresolved item must not poison the same 128 neighbors
	// forever. Bounded retries split failed aggregates, then regain throughput.
	if progressCount > 0 {
		if progressErr != nil {
			q.batchLimit = max(1, progressCount/2)
		} else {
			q.batchLimit = min(wire.MaxHourlyItems, max(q.batchLimit, progressCount)*2)
		}
	}
}

func (r *peerReceivers) runHourlyFinancial(ctx context.Context) {
	q := r.hourly.financial
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil || !r.master.currentHourlyPeer(r.peer) {
			return
		}
		work := q.take(r.master.service.now())
		if len(work) == 0 {
			select {
			case <-ctx.Done():
				return
			case <-q.wake:
			case <-ticker.C:
			}
			continue
		}
		r.applyHourlyFinancial(ctx, work)
	}
}

func (r *peerReceivers) applyHourlyFinancial(ctx context.Context, work []hourlyFinancialWork) {
	turn, finish := context.WithTimeout(ctx, 4*time.Second)
	defer finish()
	var progress []wire.HourlyCreditProgress
	for _, item := range work {
		if item.progress != nil {
			progress = append(progress, *item.progress)
		}
	}
	call, cancel := context.WithTimeout(turn, 3*time.Second)
	progressErr := r.master.receiveHourlyCreditProgress(call, r.peer, progress)
	cancel()
	if progressErr != nil {
		r.master.rejectPeerFrame(r.peer, wire.Message{Type: "hourly.demand"}, progressErr)
	}
	proofErrors := map[string]error{}
	var deferred, attempted []hourlyFinancialWork
	for _, item := range work {
		if item.proof == nil {
			attempted = append(attempted, item)
			continue
		}
		if err := turn.Err(); err != nil {
			proofErrors[item.id] = err // Never clear or ACK an unattempted proof.
			deferred = append(deferred, item)
			continue
		}
		message := wire.New("hourly.credit.proofs", "", 0, wire.HourlyCreditProofBatch{Proofs: []wire.HourlyCreditProof{*item.proof}})
		proofErrors[item.id] = r.master.receiveHourlyCreditProofs(turn, r.peer, message)
		if err := proofErrors[item.id]; err != nil {
			r.master.rejectPeerFrame(r.peer, message, err)
		}
		attempted = append(attempted, item)
	}
	// Put unattempted neighbors ahead of the slow item next time, so one locked
	// proof cannot exhaust every turn before healthy neighbors ever reach SQL.
	r.hourly.financial.finish(append(deferred, attempted...), progressErr, proofErrors, r.master.service.now())
}
