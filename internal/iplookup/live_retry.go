package iplookup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/model"
)

// liveRetryRepository repairs frozen snapshot rows and appends durable jobs.
type liveRetryRepository interface {
	IPLiveSnapshotByID(context.Context, string) (LiveDetails, string, error)
	UpdateIPLiveSnapshot(context.Context, string, LiveDetails) error
	EnqueueOutbox(context.Context, string, string, time.Time) error
}

// liveRetryPlan names the sections worth refetching. A failed topology
// cascades to every dependent section because their inputs would be stale.
type liveRetryPlan struct {
	Topology  bool
	Graph     bool
	Block     bool
	ASNs      bool
	Scores    bool
	PTR       bool
	Shodan    bool
	Transfers bool
}

func liveMetaFailed(m LiveMeta) bool {
	return m.Status == "error" || m.Status == "partial"
}

func liveRetryPlanOf(r LiveDetails) liveRetryPlan {
	var plan liveRetryPlan
	plan.Topology = liveMetaFailed(r.Topology.LiveMeta)
	plan.Graph = plan.Topology || relationshipMetaFailed(r.Topology.RelationshipMeta) || liveMetaFailed(r.Topology.ClassificationMeta)
	plan.Block = liveMetaFailed(r.Block.LiveMeta)
	plan.Transfers = r.Transfers != nil && liveMetaFailed(r.Transfers.LiveMeta)
	plan.Scores = liveMetaFailed(r.Scores.LiveMeta)
	plan.PTR = liveMetaFailed(r.PTR.LiveMeta)
	plan.Shodan = liveMetaFailed(r.Shodan.LiveMeta)
	plan.ASNs = plan.Topology
	for _, asn := range r.ASNs {
		for _, meta := range asn.Sources {
			plan.ASNs = plan.ASNs || liveMetaFailed(meta)
		}
		plan.ASNs = plan.ASNs || liveMetaFailed(asn.Traffic.BotHuman.LiveMeta) || liveMetaFailed(asn.Traffic.Devices.LiveMeta) || liveMetaFailed(asn.Traffic.IPVersion.LiveMeta)
	}
	return plan
}

func (p liveRetryPlan) empty() bool {
	return !p.Topology && !p.Graph && !p.Block && !p.ASNs && !p.Scores && !p.PTR && !p.Shodan && !p.Transfers
}

// HandleLiveRetryOutbox refetches only the failed sections of one frozen
// partial snapshot and merges them in place. It bypasses per-user admission
// because it is a serialized system repair; every outbound call still flows
// through the shared upstream queues.
func (s *LiveService) HandleLiveRetryOutbox(ctx context.Context, job model.OutboxJob) error {
	repository, ok := s.repository.(liveRetryRepository)
	if !ok {
		return nil
	}
	var payload LiveRetryPayload
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return fmt.Errorf("decode %s payload: %w", LiveRetryOutboxKind, err)
	}
	raw, err := s.settings.Optional(ctx, SettingKey)
	if err != nil {
		return &CodeError{"IP_DETAILS_UNAVAILABLE"}
	}
	c, err := DecodeConfig(raw)
	if err != nil {
		return err
	}
	if !c.Enabled || !RetryAllowed(c, payload.Attempt) {
		return nil
	}
	details, status, err := repository.IPLiveSnapshotByID(ctx, payload.SnapshotID)
	if err != nil {
		if errors.Is(err, ErrRepairGone) {
			return nil
		}
		return err
	}
	if status != "partial" {
		return nil
	}
	// Older projections require an explicit paid refresh, including queued repair jobs.
	if details.Topology.ProjectionVersion != 2 {
		return nil
	}
	plan := liveRetryPlanOf(details)
	if plan.empty() {
		return nil
	}
	attemptCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	repaired, err := s.repairLiveSections(attemptCtx, details, c, plan)
	if err != nil {
		return err
	}
	if err := repository.UpdateIPLiveSnapshot(ctx, payload.SnapshotID, repaired); err != nil {
		if errors.Is(err, ErrRepairConflict) {
			return nil
		}
		return err
	}
	return s.scheduleNextLiveRetry(ctx, repository, repaired, c, payload.Attempt)
}

// scheduleNextLiveRetry continues the chain while sections stay failed and
// the current settings policy allows another attempt.
func (s *LiveService) scheduleNextLiveRetry(ctx context.Context, repository liveRetryRepository, r LiveDetails, c Config, attempt int) error {
	if !LivePartial(r) || !RetryAllowed(c, attempt+1) {
		return nil
	}
	delay, ok := NextRetryDelay(c)
	if !ok {
		return nil
	}
	payload, err := json.Marshal(LiveRetryPayload{SnapshotID: r.SnapshotID, Attempt: attempt + 1})
	if err != nil {
		return err
	}
	return repository.EnqueueOutbox(ctx, LiveRetryOutboxKind, string(payload), s.now().UTC().Add(delay))
}

// repairLiveSections refetches planned sections concurrently. Sections write
// disjoint fields of the merged result; the topology is captured into locals
// before launch so dependent fetchers never race graph rewrites.
func (s *LiveService) repairLiveSections(ctx context.Context, details LiveDetails, c Config, plan liveRetryPlan) (LiveDetails, error) {
	r := details
	origins := r.Topology.Origins
	if plan.Topology {
		topology := s.topology(ctx, r.IP)
		origins = topology.Origins
		if len(origins) > 8 {
			origins = origins[:8]
			topology.Truncated = true
		}
		topology.ClassificationMeta = LiveMeta{Status: "empty", Source: "caida"}
		r.Topology = topology
	}
	var work sync.WaitGroup
	launch := func(fn func()) { work.Add(1); go func() { defer work.Done(); fn() }() }
	if plan.PTR {
		launch(func() { r.PTR = s.ptr(ctx, r.IP) })
	}
	if plan.Scores {
		launch(func() { r.Scores = s.scores(ctx, r.IP, c) })
	}
	if plan.Shodan {
		launch(func() { r.Shodan = s.shodan(ctx, r.IP) })
	}
	if plan.Block {
		launch(func() { r.Block = s.block(ctx, r.IP, c) })
	}
	if plan.Transfers {
		launch(func() { v := s.transfers(ctx, r.IP); r.Transfers = &v })
	}
	if plan.ASNs {
		launch(func() {
			r.ASNs = s.originDetails(ctx, origins, c)
		})
	}
	if plan.Graph {
		launch(func() {
			graph := cloneTopology(r.Topology)
			s.classify(ctx, &graph)
			s.relationships(ctx, &graph, c)
			graph = CompactTopology(graph)
			nameCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			s.resolveNames(nameCtx, &graph, c)
			cancel()
			// Replacing the node slice prevents readers seeing later name writes.
			r.Topology = cloneTopology(graph)
		})
	}
	work.Wait()
	if err := ctx.Err(); err != nil {
		return LiveDetails{}, err
	}
	return r, nil
}
