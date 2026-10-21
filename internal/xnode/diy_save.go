package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func (s *Service) SaveOwnedDIY(ctx context.Context, user, key string, input DIYWrite) (DIYSaveResult, error) {
	result := DIYSaveResult{}
	if key == "" || len(key) > 128 || input.Choices == nil || input.SettingsRevision < 0 || input.SettingsRevision >= 9007199254740991 {
		return result, ErrInvalid
	}
	repo, ok := s.repo.(diyRepository)
	if !ok {
		return result, ErrUnavailable
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return result, err
	}
	digest := sha256.Sum256(raw)
	fingerprint := hex.EncodeToString(digest[:])
	if receipt, replayed, routed, err := repo.ReplayXNodeDIY(ctx, user, key, fingerprint); err != nil || replayed {
		return DIYSaveResult{SettingsRevision: input.SettingsRevision + 1, RoutingChanged: routed, Receipt: receipt}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// Another caller can commit this key while this request waits for the lock.
	if receipt, replayed, routed, err := repo.ReplayXNodeDIY(ctx, user, key, fingerprint); err != nil || replayed {
		return DIYSaveResult{SettingsRevision: input.SettingsRevision + 1, RoutingChanged: routed, Receipt: receipt}, err
	}
	preview, err := s.PreviewOwnedDIY(ctx, user, input)
	if err != nil {
		return result, err
	}
	if !preview.Valid {
		return result, ErrInvalid
	}
	routing, err := s.diyRouting(ctx, user)
	if err != nil {
		return result, err
	}
	if routing.Plan.Revision != input.RoutingRevision {
		return result, ErrConflict
	}
	previous, err := json.Marshal(routing.Plan.Choices)
	if err != nil {
		return result, err
	}
	next, err := json.Marshal(input.Choices)
	if err != nil {
		return result, err
	}
	var plan *RoutingPlan
	items := []providerops.ItemInput{{Key: "settings", TargetType: "member_diy", TargetID: user}}
	if string(previous) != string(next) {
		identity, err := s.repo.XNodeIdentityByUser(ctx, user)
		if err != nil {
			return result, err
		}
		prepared, err := s.prepareOwnedRouting(ctx, identity, input.RoutingRevision, input.Choices)
		if err != nil {
			return result, err
		}
		plan = &prepared
		items = []providerops.ItemInput{{Key: "routing", TargetType: "xnode_user", TargetID: fmt.Sprint(identity.ProxyUserID)}}
	}
	cipher, err := s.seal("xnode.diy."+user, input.Overrides)
	if err != nil {
		return result, err
	}
	receipt, err := repo.SaveXNodeDIY(ctx, DIYDocument{UserID: user, Revision: input.SettingsRevision, Cipher: cipher}, input.RoutingRevision, plan, providerops.CreateInput{ActorUserID: user, OwnerUserID: user, Kind: DIYOperationKind, IdempotencyKey: key, RequestFingerprint: fingerprint, Items: items}, s.now())
	return DIYSaveResult{SettingsRevision: input.SettingsRevision + 1, RoutingChanged: plan != nil, Receipt: receipt}, err
}
