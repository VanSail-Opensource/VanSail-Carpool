package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func (s *Service) refreshDDNSRoutes(ctx context.Context, state DDNSChangeState, actor string) (bool, error) {
	repo, ok := s.routingRepository()
	if !ok {
		return false, ErrUnavailable
	}
	plans, err := repo.XNodeRoutingPlans(ctx)
	if err != nil {
		return false, err
	}
	ready := true
	nodes := []string{}
	for _, plan := range plans {
		affected := false
		for _, cipher := range []string{plan.Cipher, plan.PreviousCipher} {
			if cipher == "" {
				continue
			}
			copy := plan
			copy.Cipher = cipher
			doc, err := s.routingDocument(copy)
			if err != nil {
				return false, err
			}
			if routingDocumentTouchesNode(doc, state.NodeID) || slices.Contains(doc.LegacyNodes, state.NodeID) {
				affected = true
			}
		}
		if !affected {
			continue
		}
		involved, err := s.routingPlanNodes(plan)
		if err != nil {
			return false, err
		}
		nodes = append(nodes, involved...)
		if plan.Phase != "active" && plan.Phase != "disabled" {
			ready = false
			continue
		}
		doc, err := s.routingDocument(plan)
		if err != nil {
			return false, err
		}
		identity, err := s.repo.XNodeIdentity(ctx, plan.ProxyUserID)
		if err != nil {
			return false, err
		}
		input, err := s.routingInput(ctx, identity, doc.Choices)
		if err != nil {
			if plan.Phase == "active" {
				ready = false
			}
			continue
		}
		if input.dependencies == doc.Dependencies {
			continue
		}
		key := sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%s", plan.ProxyUserID, plan.Revision, input.dependencies)))
		requestKey := fmt.Sprintf("routing-refresh-%x", key[:20])
		if identity.UserID != "" {
			_, err = s.QueueOwnedRouting(ctx, identity.UserID, requestKey, plan.Revision, doc.Choices)
		} else {
			// Standalone plans use the same graph preparation and barriers,
			// attributed to the DDNS administrator without inventing a member.
			var prepared RoutingPlan
			prepared, err = s.prepareOwnedRouting(ctx, identity, plan.Revision, doc.Choices)
			if err == nil {
				raw, e := json.Marshal([]any{plan.Revision, doc.Choices})
				if e != nil {
					return false, e
				}
				fingerprint := sha256.Sum256(raw)
				_, err = repo.QueueXNodeRouting(ctx, prepared, providerops.CreateInput{ActorUserID: actor, Kind: RoutingOperationKind, IdempotencyKey: requestKey, RequestFingerprint: hex.EncodeToString(fingerprint[:]), Items: []providerops.ItemInput{{Key: "routing", TargetType: "xnode_user", TargetID: fmt.Sprint(identity.ProxyUserID)}}}, s.now().UTC())
			}
		}
		if err != nil && !errors.Is(err, ErrConflict) {
			return false, err
		}
		ready = false
	}
	if !ready {
		return false, nil
	}
	slices.Sort(nodes)
	nodes = slices.Compact(nodes)
	// Confirm disabled graphs too before dropping their old bypass addresses.
	for _, node := range nodes {
		if !s.master.Connected(node) {
			return false, nil
		}
		if err := s.master.Sync(ctx, node); err != nil {
			return false, err
		}
		child, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
		err := s.master.WaitApplied(child, node)
		cancel()
		if err != nil {
			return false, nil
		}
	}
	return true, nil
}
