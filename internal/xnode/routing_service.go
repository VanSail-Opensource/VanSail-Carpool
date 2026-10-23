package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/routingmatch"
	"time"
)

func (s *Service) OwnedRoutingPlan(ctx context.Context, user string) (RoutingView, error) {
	view := RoutingView{Plan: RoutingPlan{Choices: []RoutingChoice{}, Phase: "active"}, Nodes: []RoutingNode{}, Templates: []RoutingTemplate{}, Paths: []routingmatch.Path{}}
	identity, err := s.repo.XNodeIdentityByUser(ctx, user)
	if err != nil {
		return view, err
	}
	repo, ok := s.routingRepository()
	if !ok {
		return view, ErrUnavailable
	}
	plan, err := repo.XNodeRoutingPlan(ctx, identity.ProxyUserID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return view, err
	}
	if err == nil {
		doc, err := s.routingDocument(plan)
		if err != nil {
			return view, err
		}
		plan.Choices = doc.Choices
		if plan.Choices == nil {
			plan.Choices = []RoutingChoice{}
		}
		view.Plan = plan
	}
	catalog, err := s.RoutingTemplateConfig(ctx)
	if err != nil {
		return view, err
	}
	view.Templates = catalog.Templates
	in, err := s.routingInput(ctx, identity, nil)
	if err != nil {
		return view, err
	}
	view.Nodes = in.nodes
	view.Endpoints = in.available
	if len(view.Plan.Choices) > 0 {
		in, err = s.routingInput(ctx, identity, view.Plan.Choices)
		if err == nil {
			view.Plan.Choices = in.choices
			view.Paths, err = in.graph.Paths()
			view.Paths = in.publicPaths(view.Paths)
		}
		if err != nil {
			view.Plan.Reason = "routing_dependencies_changed"
		}
	}
	return view, nil
}
func (s *Service) QueueOwnedRouting(ctx context.Context, user, key string, revision int64, choices []RoutingChoice) (model.OperationReceipt, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if choices == nil {
		choices = []RoutingChoice{}
	}
	// Preparation is read-only. The store fences replay and revision atomically;
	// waiting for agent validation must not hold the global mutation mutex.
	identity, err := s.repo.XNodeIdentityByUser(ctx, user)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	repo, ok := s.routingRepository()
	if !ok {
		return model.OperationReceipt{}, ErrUnavailable
	}
	if key == "" || len(key) > 128 {
		return model.OperationReceipt{}, ErrInvalid
	}
	raw, err := json.Marshal([]any{revision, choices})
	if err != nil {
		return model.OperationReceipt{}, err
	}
	hash := sha256.Sum256(raw)
	fingerprint := hex.EncodeToString(hash[:])
	if receipt, replayed, err := repo.ReplayXNodeRouting(ctx, user, key, fingerprint); err != nil || replayed {
		return receipt, err
	}
	plan, err := s.prepareOwnedRouting(ctx, identity, revision, choices)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	input := providerops.CreateInput{ActorUserID: user, OwnerUserID: user, Kind: RoutingOperationKind, IdempotencyKey: key, RequestFingerprint: fingerprint, Items: []providerops.ItemInput{{Key: "routing", TargetType: "xnode_user", TargetID: fmt.Sprint(identity.ProxyUserID)}}}
	return repo.QueueXNodeRouting(ctx, plan, input, s.now().UTC())
}
func (s *Service) archiveLegacyRouting(ctx context.Context, identity Identity) (string, error) {
	nodes, err := s.Nodes(ctx)
	if err != nil {
		return "", err
	}
	saved := []Customization{}
	for _, n := range nodes {
		items, err := s.repo.XNodeCustomizations(ctx, n.ID)
		if err != nil {
			return "", err
		}
		for _, item := range items {
			if item.ProxyUserID == identity.ProxyUserID {
				saved = append(saved, item)
			}
		}
	}
	// Marshal the private evidence explicitly: public Customization deliberately
	// omits its ciphertexts. Evidence stays encrypted and is never returned.
	evidence := []map[string]any{}
	for _, item := range saved {
		evidence = append(evidence, map[string]any{"nodeId": item.NodeID, "revision": item.Revision, "outboundsCipher": item.OutboundsCipher, "routingCipher": item.RoutingCipher})
	}
	return s.seal(fmt.Sprintf("xnode.routes.legacy.%d", identity.ProxyUserID), evidence)
}
