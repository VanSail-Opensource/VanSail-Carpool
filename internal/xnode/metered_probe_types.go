package xnode

import (
	"context"
	"time"
)

// MeteredProbe is immutable attribution, retained after access expires/revokes.
// FlushedSequence proves the journal barrier used before financial settlement.
type MeteredProbe struct {
	ProxyUserID                                                    int64
	PeriodID, OperationID, PairKey, EndpointID, NodeID, InboundTag string
	ValidUntil                                                     time.Time
	Revoked                                                        bool
	FlushedSequence                                                *uint64
}

type probeRepository interface {
	XNodeMeteredProbe(context.Context, int64) (MeteredProbe, error)
	XNodePeriodProbes(context.Context, string) ([]MeteredProbe, error)
	SaveXNodeMeteredProbe(context.Context, MeteredProbe) error
	RevokeXNodeMeteredProbe(context.Context, int64, time.Time) error
	SealXNodeMeteredProbe(context.Context, int64, uint64) error
}

func (s *Service) probeRepository() (probeRepository, bool) {
	base := s.repo
	for {
		switch r := base.(type) {
		case *compileReads:
			base = r.Repository
		case candidateRepository:
			base = r.Repository
		default:
			value, ok := base.(probeRepository)
			return value, ok
		}
	}
}
