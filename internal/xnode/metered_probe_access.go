package xnode

import (
	"context"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (s *Service) meteredProbeUser(ctx context.Context, identity Identity, node Node, user wire.User, inbounds map[string]map[string]any) (wire.User, error) {
	if identity.AccessDisabled {
		return user, nil
	}
	repo, ok := s.probeRepository()
	if !ok {
		return user, ErrUnavailable
	}
	p, err := repo.XNodeMeteredProbe(ctx, identity.ProxyUserID)
	if isMissing(err) {
		return user, nil
	}
	if err != nil {
		return user, err
	}
	if p.Revoked || p.NodeID != node.ID || !p.ValidUntil.After(s.now()) {
		return user, nil
	}
	period, err := s.repo.XNodePeriod(ctx, p.PeriodID)
	if err != nil {
		return user, err
	}
	owner, err := s.repo.XNodeIdentity(ctx, period.ProxyUserID)
	if err != nil {
		return user, err
	}
	if !meteredProbePeriodEligible(p, period, owner, s.now()) {
		return user, nil
	}
	tags, eligible, err := s.eligibleTags(ctx, owner.UserID, node.ID)
	if err != nil {
		return user, err
	}
	allowed := false
	for _, tag := range tags {
		allowed = allowed || tag == p.InboundTag
	}
	if !allowed || eligible == nil || eligible.ID != p.PeriodID {
		return user, nil
	}
	purchase, err := s.EffectivePurchase(ctx, owner.UserID)
	if err != nil {
		return user, err
	}
	if purchase == nil {
		return user, nil
	}
	until, err := s.continuityUntil(ctx, *purchase)
	if err != nil {
		return user, err
	}
	if until.Before(p.ValidUntil) {
		p.ValidUntil = until
	}
	credentials, err := loadCredentials(s.vault, identity)
	if err != nil {
		return user, err
	}
	inbound := inbounds[p.InboundTag]
	if inbound == nil {
		return user, ErrInvalid
	}
	raw, err := inboundCredential(inbound, credentials)
	if err != nil {
		return user, err
	}
	user.InboundTags = []string{p.InboundTag}
	user.UUID = credentials.UUID
	user.Credentials[p.InboundTag] = raw
	user.AccountingPeriod = p.PeriodID
	user.ValidUntil = &p.ValidUntil
	user.ConnectionLimit, user.SpeedLimitMbps = 0, 0
	return user, nil
}
