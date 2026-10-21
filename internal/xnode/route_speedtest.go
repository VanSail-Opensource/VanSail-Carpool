package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func (s *Service) QueueRouteSpeedtest(ctx context.Context, user, key string, request RouteSpeedtestRequest) (model.OperationReceipt, error) {
	repo, ok := s.repo.(routeSpeedtestRepository)
	if !ok {
		return model.OperationReceipt{}, ErrUnavailable
	}
	if key == "" || len(key) > 128 || request.Start == "" || request.StartEndpointID == "" {
		return model.OperationReceipt{}, ErrInvalid
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	hash := sha256.Sum256(raw)
	fingerprint := hex.EncodeToString(hash[:])
	if receipt, replayed, err := repo.ReplayXNodeRouteSpeedtest(ctx, user, key, fingerprint); replayed || err != nil {
		return receipt, err
	}
	doc, err := s.routeSpeedtestPlan(ctx, user, request)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	sealed, err := s.seal("xnode.route-speedtest."+user+"."+key, doc)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	items := make([]providerops.ItemInput, 0, len(doc.Pairs))
	for i, pair := range doc.Pairs {
		items = append(items, providerops.ItemInput{Key: fmt.Sprintf("%02d", i), TargetType: "xnode_speedtest", TargetID: speedtestPairTarget(pair)})
	}
	op, _, err := repo.CreateProviderOperation(ctx, providerops.CreateInput{ActorUserID: user, OwnerUserID: user, Kind: RouteSpeedtestOperation, IdempotencyKey: key, RequestFingerprint: fingerprint, SealedTarget: sealed, Items: items}, s.now())
	return op.Receipt, err
}

// Revalidate the saved desired route and its trace both at submission and before
// each pair. Draft graphs and stale dependency digests never authorize probes.
func (s *Service) routeSpeedtestPlan(ctx context.Context, user string, request RouteSpeedtestRequest) (routeSpeedtestDocument, error) {
	doc := routeSpeedtestDocument{Request: request}
	identity, err := s.repo.XNodeIdentityByUser(ctx, user)
	if err != nil {
		return doc, err
	}
	routes, ok := s.routingRepository()
	if !ok || s.master == nil {
		return doc, ErrUnavailable
	}
	plan, err := routes.XNodeRoutingPlan(ctx, identity.ProxyUserID)
	if err != nil {
		return doc, err
	}
	if plan.Phase != "active" || plan.Revision != request.Revision || identity.AccessDisabled {
		return doc, ErrConflict
	}
	saved, err := s.routingDocument(plan)
	if err != nil {
		return doc, err
	}
	in, err := s.routingInput(ctx, identity, saved.Choices)
	if err != nil {
		return doc, err
	}
	if in.dependencies != saved.Dependencies {
		return doc, ErrConflict
	}
	_, trace, err := s.PreviewOwnedRouting(ctx, user, saved.Choices, request.Start, request.Destination, request.StartEndpointID)
	if err != nil {
		return doc, err
	}
	if trace == nil || len(trace.Rules) == 0 || len(trace.Nodes) != len(trace.Rules)+1 {
		return doc, ErrInvalid
	}
	period, err := s.repo.XNodePeriod(ctx, identity.CurrentPeriodID)
	if err != nil {
		return doc, err
	}
	if period.Status != "open" || period.ProxyUserID != identity.ProxyUserID || period.LimitBytes > 0 && period.UsedBytes >= period.LimitBytes {
		return doc, ErrConflict
	}
	doc.PeriodID = period.ID
	for index, id := range trace.Rules {
		source, target := in.sources[id], in.targets[id]
		if source.ID == "" || target.ID == "" || source.NodeID != trace.Nodes[index] || target.NodeID != trace.Nodes[index+1] {
			return doc, ErrInvalid
		}
		for _, node := range []string{source.NodeID, target.NodeID} {
			if !s.master.updateReady(node, s.master.Version(node)) {
				return doc, ErrUnavailable
			}
			for _, capability := range []string{"traffic_periods_v1", "traffic_period_barriers_v1"} {
				if !slices.Contains(s.master.Capabilities(node), capability) {
					return doc, &CapabilityError{Capability: capability}
				}
			}
		}
		doc.Pairs = append(doc.Pairs, RouteSpeedtestPair{SourceNodeID: source.NodeID, TargetEndpointID: target.ID})
	}
	global, err := s.Global(ctx)
	if err != nil {
		return doc, err
	}
	if global.SpeedtestDownloadURL == "" || global.SpeedtestUploadURL == "" {
		return doc, ErrInvalid
	}
	return doc, nil
}
