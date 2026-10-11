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
	finished map[string]ProgressTask
	active   map[string]bool
	emit     func(LiveEvent)
}

// start marks phases as running and reports the transition to subscribers.
func (p *livePipeline) start(phases ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, phase := range phases {
		p.active[phase] = true
	}
	p.emit(LiveEvent{Type: "progress", IP: p.result.IP, Progress: p.progress()})
}

// progress projects per-phase tasks: finished sections keep their verdict,
// launched-but-unfinished sections are active, and the rest stay pending.
func (p *livePipeline) progress() *Progress {
	progress := &Progress{Total: len(livePhases), Phase: "complete", Tasks: make([]ProgressTask, 0, len(livePhases))}
	for _, stage := range livePhases {
		task := ProgressTask{Key: stage, Status: "pending"}
		if finished, ok := p.finished[stage]; ok {
			task = finished
			progress.Completed++
		} else {
			if p.active[stage] {
				task.Status = "active"
			}
			if progress.Phase == "complete" {
				progress.Phase = stage
			}
		}
		progress.Tasks = append(progress.Tasks, task)
	}
	// Only the final complete event reports 100%; settlement is still pending here.
	progress.Completed = min(progress.Completed, len(livePhases)-1)
	return progress
}

func (p *livePipeline) update(phase, section string, value any, apply func(*LiveDetails)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if apply != nil {
		apply(&p.result)
	}
	if phase != "" {
		p.finished[phase] = liveTaskStatus(phase, value)
		delete(p.active, phase)
	}
	if section != "" {
		if graph, ok := value.(BGPTopology); ok {
			value = CompactTopology(graph)
		}
		data, err := json.Marshal(value)
		if err == nil {
			p.emit(LiveEvent{Type: "section", IP: p.result.IP, Section: section, Data: data})
		}
	}
	p.emit(LiveEvent{Type: "progress", IP: p.result.IP, Progress: p.progress()})
}

func cloneTopology(t BGPTopology) BGPTopology {
	t.Nodes = append([]BGPNode{}, t.Nodes...)
	t.Edges = append([]BGPEdge{}, t.Edges...)
	t.Origins = append([]uint32{}, t.Origins...)
	return t
}

func (s *LiveService) livePipeline(ctx context.Context, ip string, c Config, emit func(LiveEvent)) LiveDetails {
	p := &livePipeline{result: LiveDetails{IP: ip, FetchedAt: s.now().UTC().Format(time.RFC3339Nano), ASNs: []ASNDetails{}}, finished: map[string]ProgressTask{}, active: map[string]bool{}, emit: emit}
	for _, phase := range []string{"ptr", "scores", "shodan", "topology", "block", "transfers"} {
		p.active[phase] = true
	}
	p.update("", "", nil, nil)
	var work sync.WaitGroup
	launch := func(fn func()) { work.Add(1); go func() { defer work.Done(); fn() }() }
	launch(func() { v := s.ptr(ctx, ip); p.update("ptr", "ptr", v, func(r *LiveDetails) { r.PTR = v }) })
	launch(func() {
		v := s.scores(ctx, ip, c)
		p.update("scores", "scores", v, func(r *LiveDetails) { r.Scores = v })
	})
	launch(func() { v := s.shodan(ctx, ip); p.update("shodan", "shodan", v, func(r *LiveDetails) { r.Shodan = v }) })
	launch(func() { v := s.block(ctx, ip, c); p.update("block", "block", v, func(r *LiveDetails) { r.Block = v }) })
	launch(func() {
		v := s.transfers(ctx, ip)
		p.update("transfers", "transfers", v, func(r *LiveDetails) { r.Transfers = &v })
	})
	topology := s.topology(ctx, ip)
	origins := topology.Origins
	if len(origins) > 8 {
		origins = origins[:8]
		topology.Truncated = true
	}
	topology.ClassificationMeta = LiveMeta{Status: "empty", Source: "caida"}
	p.update("topology", "topology", topology, func(r *LiveDetails) { r.Topology = topology })
	p.start("asns", "relationships")
	launch(func() {
		values := s.originDetails(ctx, origins, c)
		p.update("asns", "asns", values, func(r *LiveDetails) { r.ASNs = values })
	})
	launch(func() {
		graph := cloneTopology(topology)
		s.classify(ctx, &graph)
		classified := cloneTopology(graph)
		p.update("", "topology", classified, func(r *LiveDetails) { r.Topology = classified })
		s.relationships(ctx, &graph, c)
		p.update("relationships", "topology", graph, func(r *LiveDetails) { r.Topology = graph })
		p.start("names")
		graph = CompactTopology(graph)
		nameCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		s.resolveNames(nameCtx, &graph, c)
		cancel()
		// Replacing the node slice prevents readers seeing subsequent name writes.
		graph = cloneTopology(graph)
		p.update("names", "topology", graph, func(r *LiveDetails) { r.Topology = graph })
	})
	work.Wait()
	return CompactLiveDetails(p.result)
}
