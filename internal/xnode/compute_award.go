package xnode

import (
	"context"
	"sync"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (p *computePool) award(ctx context.Context, a *computeAttempt, excluded map[*agentPeer]bool) (*agentPeer, error) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		if !p.pressure.Load() {
			return nil, errComputeLocal
		}
		p.master.mu.RLock()
		peers := []*agentPeer{}
		for id, peer := range p.master.peers {
			live := p.master.live[id]
			if !excluded[peer] && peer.computePool && !live.receivedAt.IsZero() && time.Since(live.receivedAt) < time.Minute && live.report.Ready && live.report.Synchronized {
				peers = append(peers, peer)
			}
		}
		p.master.mu.RUnlock()
		announce, cancel := context.WithTimeout(ctx, time.Second)
		var group sync.WaitGroup
		next := make(chan *agentPeer)
		for range min(32, len(peers)) {
			group.Add(1)
			go func() {
				defer group.Done()
				for peer := range next {
					p.mu.Lock()
					busy := p.busy[peer] != ""
					p.mu.Unlock()
					if !busy {
						_ = peer.send(announce, wire.New("compute.announce", a.offer.Signal.BatchID, 0, a.offer))
					}
				}
			}()
		}
		// Broadcast independently of socket readers, with bounded fan-out and one
		// existing serialized writer per connection. Slow peers cannot delay claims.
		sent := make(chan struct{})
		go func() {
			defer close(sent)
			defer close(next)
			for _, peer := range peers {
				select {
				case next <- peer:
				case <-announce.Done():
					return
				}
			}
		}()
		select {
		case peer := <-a.claimed:
			cancel()
			<-sent
			group.Wait()
			return p.grant(ctx, a, peer)
		case <-announce.Done():
		}
		cancel()
		<-sent
		group.Wait()

		select {
		case peer := <-a.claimed:
			return p.grant(ctx, a, peer)
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-tick.C:
		}
	}
}
