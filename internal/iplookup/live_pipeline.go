package iplookup

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

type livePipeline struct {
	mu       sync.Mutex
	result   LiveDetails
	finished map[string]bool
	emit     func(LiveEvent)
}

func (p *livePipeline) update(phase, section string, value any, apply func(*LiveDetails)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if apply != nil {
		apply(&p.result)
	}
	if phase != "" {
		p.finished[phase] = true
	}
	if section != "" {
		data, err := json.Marshal(value)
		if err == nil {
			p.emit(LiveEvent{Type: "section", IP: p.result.IP, Section: section, Data: data})
		}
	}
	progress := &Progress{Total: len(livePhases), Phase: "complete"}
	for _, stage := range livePhases {
		if p.finished[stage] {
			progress.Completed++
		} else if progress.Phase == "complete" {
			progress.Phase = stage
		}
	}
	// Only the final complete event reports 100%; settlement is still pending here.
	progress.Completed = min(progress.Completed, len(livePhases)-1)
	p.emit(LiveEvent{Type: "progress", IP: p.result.IP, Progress: progress})
}

func cloneTopology(t BGPTopology) BGPTopology {
	t.Nodes = append([]BGPNode{}, t.Nodes...)
	t.Edges = append([]BGPEdge{}, t.Edges...)
	t.Origins = append([]uint32{}, t.Origins...)
	return t
}

func (s *LiveService) livePipeline(ctx context.Context, ip string, c Config, emit func(LiveEvent)) LiveDetails {
	p := &livePipeline{result: LiveDetails{IP: ip, FetchedAt: s.now().UTC().Format(time.RFC3339), ASNs: []ASNDetails{}}, finished: map[string]bool{}, emit: emit}
	p.update("", "", nil, nil)
	var work sync.WaitGroup
	launch := func(fn func()) { work.Add(1); go func() { defer work.Done(); fn() }() }
	launch(func() { v := s.ptr(ctx, ip); p.update("ptr", "ptr", v, func(r *LiveDetails) { r.PTR = v }) })
	launch(func() {
		v := s.scores(ctx, ip, c)
		p.update("scores", "scores", v, func(r *LiveDetails) { r.Scores = v })
	})
	launch(func() { v := s.shodan(ctx, ip); p.update("shodan", "shodan", v, func(r *LiveDetails) { r.Shodan = v }) })
	topology := s.topology(ctx, ip)
	origins := topology.Origins
	if len(origins) > 8 {
		origins = origins[:8]
		topology.Truncated = true
	}
	topology.ClassificationMeta = LiveMeta{Status: "empty", Source: "caida"}
	p.update("topology", "topology", topology, func(r *LiveDetails) { r.Topology = topology })
	launch(func() {
		v := s.block(ctx, topology.Prefix, c)
		p.update("block", "block", v, func(r *LiveDetails) { r.Block = v })
	})
	launch(func() {
		values := make([]ASNDetails, len(origins))
		var asnWork sync.WaitGroup
		for i, asn := range origins {
			asnWork.Add(1)
			go func() { defer asnWork.Done(); values[i] = s.asn(ctx, asn, c) }()
		}
		asnWork.Wait()
		p.update("asns", "asns", values, func(r *LiveDetails) { r.ASNs = values })
	})
	launch(func() {
		graph := cloneTopology(topology)
		s.classify(ctx, &graph)
		s.relationships(ctx, &graph, c)
		p.update("relationships", "topology", graph, func(r *LiveDetails) { r.Topology = graph })
		nameCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		s.resolveNames(nameCtx, &graph, c)
		cancel()
		// Replacing the node slice prevents readers seeing subsequent name writes.
		graph = cloneTopology(graph)
		p.update("names", "topology", graph, func(r *LiveDetails) { r.Topology = graph })
	})
	work.Wait()
	return p.result
}
