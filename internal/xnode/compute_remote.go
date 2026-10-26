package xnode

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/X-Node/pkg/compute"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

var errComputeLocal = errors.New("master calculation pressure cleared")

func (p *computePool) remote(ctx context.Context, b compute.Batch) (compute.Results, func(bool), error) {
	raw, hash, err := compute.DigestBatch(b)
	if err != nil {
		return compute.Results{}, nil, err
	}
	id, err := ids.New()
	if err != nil {
		return compute.Results{}, nil, err
	}
	excluded := map[*agentPeer]bool{}
	for attempt := 1; ctx.Err() == nil; attempt++ {
		a := &computeAttempt{input: raw, claimed: make(chan *agentPeer, 1), chunks: make(chan wire.ComputeChunk, 24)}
		deadline, _ := ctx.Deadline()
		a.offer = wire.ComputeAnnouncement{Signal: wire.ComputeSignal{BatchID: id, Attempt: attempt, InputHash: hash}, Version: compute.Version, Items: len(b.Tasks), Bytes: len(raw), Deadline: deadline.Unix()}
		p.mu.Lock()
		p.attempts[id] = a
		p.mu.Unlock()
		peer, err := p.award(ctx, a, excluded)
		if err != nil {
			p.mu.Lock()
			winner := a.winner
			p.mu.Unlock()
			if winner != nil {
				excluded[winner] = true
				p.finish(a, winner, false)
			} else {
				p.release(a)
			}
			if ctx.Err() != nil {
				return compute.Results{}, nil, ctx.Err()
			}
			if errors.Is(err, errComputeLocal) {
				return compute.Results{}, nil, err
			}
			continue
		}
		result, err := p.collect(ctx, a, peer)
		if err == nil {
			err = compute.ValidateResult(b, result)
		}
		if err != nil {
			excluded[peer] = true
			p.finish(a, peer, false)
			if ctx.Err() != nil {
				return compute.Results{}, nil, ctx.Err()
			}
			continue
		}
		p.mu.Lock()
		a.uploadComplete = true
		p.mu.Unlock()
		return result, func(accepted bool) { p.finish(a, peer, accepted) }, nil
	}
	return compute.Results{}, nil, ctx.Err()
}

func (p *computePool) release(a *computeAttempt) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.attempts[a.offer.Signal.BatchID] == a {
		delete(p.attempts, a.offer.Signal.BatchID)
	}
	if a.winner != nil && p.busy[a.winner] == a.offer.Signal.BatchID {
		delete(p.busy, a.winner)
	}
}

func (p *computePool) finish(a *computeAttempt, peer *agentPeer, accepted bool) {
	if accepted {
		p.mu.Lock()
		for key, r := range p.accepted {
			if time.Now().After(r.until) || len(p.accepted) >= 256 {
				delete(p.accepted, key)
			}
		}
		p.accepted[a.offer.Signal] = computeReceipt{peer: peer, until: time.Now().Add(5 * time.Minute)}
		p.mu.Unlock()
	}
	typ := "compute.cancel"
	if accepted {
		typ = "compute.ack"
	}
	ctx, cancel := context.WithTimeout(peer.ctx, time.Second)
	_ = peer.send(ctx, wire.New(typ, a.offer.Signal.BatchID, 0, a.offer.Signal))
	cancel()
	p.release(a)
}

func (p *computePool) grant(ctx context.Context, a *computeAttempt, peer *agentPeer) (*agentPeer, error) {
	if err := peer.send(ctx, wire.New("compute.grant", a.offer.Signal.BatchID, 0, wire.ComputeGrant{Offer: a.offer, Input: a.input})); err != nil {
		return nil, err
	}
	return peer, nil
}
