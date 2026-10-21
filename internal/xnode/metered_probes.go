package xnode

import (
	"context"
	"strings"
	"time"
)

const memberProbePrefix = "member-probe-"

func (s *Service) prepareMeteredProbe(ctx context.Context, operation, key string, period Period, endpoint Endpoint) (Identity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	repo, ok := s.probeRepository()
	if !ok {
		return Identity{}, ErrUnavailable
	}
	name := memberProbePrefix + operation + "-" + key
	identities, err := s.repo.XNodeIdentities(ctx)
	if err != nil {
		return Identity{}, err
	}
	for _, identity := range identities {
		if identity.ServiceName != name {
			continue
		}
		p, err := repo.XNodeMeteredProbe(ctx, identity.ProxyUserID)
		if err == nil {
			if p.PeriodID != period.ID || p.EndpointID != endpoint.ID || p.Revoked || !p.ValidUntil.After(s.now().Add(65*time.Second)) {
				return identity, ErrConflict
			}
			return identity, nil
		}
		if !isMissing(err) {
			return identity, err
		}
		return identity, s.bindMeteredProbe(ctx, repo, identity, operation, key, period, endpoint)
	}
	identity := Identity{ServiceName: name}
	credentials, err := NewCredentials()
	if err != nil {
		return identity, err
	}
	identity.CredentialsCipher, err = sealCredentials(s.vault, identity, credentials)
	if err != nil {
		return identity, err
	}
	identity, err = s.repo.SaveXNodeIdentity(ctx, identity)
	if err != nil {
		return identity, err
	}
	return identity, s.bindMeteredProbe(ctx, repo, identity, operation, key, period, endpoint)
}

func (s *Service) bindMeteredProbe(ctx context.Context, repo probeRepository, identity Identity, operation, key string, period Period, endpoint Endpoint) error {
	return repo.SaveXNodeMeteredProbe(ctx, MeteredProbe{ProxyUserID: identity.ProxyUserID, PeriodID: period.ID, OperationID: operation, PairKey: key, EndpointID: endpoint.ID, NodeID: endpoint.NodeID, InboundTag: endpoint.InboundTag, ValidUntil: s.now().Add(2 * time.Minute)})
}

func (s *Service) flushMeteredProbe(ctx context.Context, p MeteredProbe) error {
	if p.FlushedSequence != nil {
		return nil
	}
	repo, ok := s.probeRepository()
	if !ok || s.master == nil {
		return ErrUnavailable
	}
	if err := repo.RevokeXNodeMeteredProbe(ctx, p.ProxyUserID, s.now()); err != nil {
		return err
	}
	if !s.master.Connected(p.NodeID) {
		return ErrUnavailable
	}
	if err := s.master.Sync(ctx, p.NodeID); err != nil {
		return err
	}
	if err := s.master.WaitApplied(ctx, p.NodeID); err != nil {
		return err
	}
	barrier, err := s.master.FlushPeriod(ctx, p.NodeID, p.ProxyUserID, p.PeriodID)
	if err != nil {
		return err
	}
	return repo.SealXNodeMeteredProbe(ctx, p.ProxyUserID, barrier.Sequence)
}

func isMeteredProbe(identity Identity) bool {
	return strings.HasPrefix(identity.ServiceName, memberProbePrefix)
}

func meteredProbePeriodEligible(p MeteredProbe, period Period, owner Identity, now time.Time) bool {
	return !p.Revoked && p.ValidUntil.After(now) && !owner.AccessDisabled && owner.CurrentPeriodID == p.PeriodID && period.Status == "open" && (period.LimitBytes == 0 || period.UsedBytes < period.LimitBytes)
}

func (s *Service) revokePeriodProbes(ctx context.Context, period string) error {
	repo, ok := s.probeRepository()
	if !ok {
		return nil
	}
	probes, err := repo.XNodePeriodProbes(ctx, period)
	if err != nil {
		return err
	}
	for _, p := range probes {
		if !p.Revoked {
			if err := repo.RevokeXNodeMeteredProbe(ctx, p.ProxyUserID, s.now()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) flushPeriodProbes(ctx context.Context, period, node string) error {
	repo, ok := s.probeRepository()
	if !ok {
		return nil
	}
	probes, err := repo.XNodePeriodProbes(ctx, period)
	if err != nil {
		return err
	}
	for _, probe := range probes {
		if probe.NodeID == node {
			if err := s.flushMeteredProbe(ctx, probe); err != nil {
				return err
			}
		}
	}
	return nil
}
