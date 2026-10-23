package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func (s *Service) SaveOwnedDIY(ctx context.Context, user, key string, input DIYWrite) (DIYSaveResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
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
	if replay, found, err := repo.ReplayXNodeDIYResult(ctx, user, key, fingerprint); err != nil || found {
		if found && replay.Legacy {
			replay.SettingsRevision = input.SettingsRevision + 1
			replay.Changed = true
		}
		return replay, err
	}
	// The store atomically fences revisions, quotes, charges and replay. Keep
	// read-only preparation and agent waits outside the global mutation mutex.
	preview, err := s.PreviewOwnedDIY(ctx, user, input)
	if err != nil {
		return result, err
	}
	if !preview.Valid {
		return result, ErrInvalid
	}
	proxies, err := s.diyProxies(ctx, user)
	if err != nil {
		return result, err
	}
	previousSettings, settings, err := s.diySettingsForWrite(ctx, user, input, proxies)
	if err != nil {
		return result, err
	}
	whitelistChanged := previousSettings.Whitelist.Enabled != settings.Whitelist.Enabled || settings.Whitelist.Enabled && !sameDIYValue(previousSettings.Whitelist, settings.Whitelist)
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
	if whitelistChanged {
		items = []providerops.ItemInput{{Key: "whitelist", TargetType: "member_whitelist", TargetID: user}}
	}
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
	cipher, err := s.seal("xnode.diy."+user, settings)
	if err != nil {
		return result, err
	}
	changed := !sameDIYValue(previousSettings, settings) || plan != nil
	result = DIYSaveResult{Changed: changed, SettingsRevision: input.SettingsRevision, RoutingChanged: plan != nil, DeploymentChanged: plan != nil || whitelistChanged}
	var quote *DIYBillingQuote
	if changed {
		result.SettingsRevision++
		if _, ok := s.repo.(diyBillingRepository); ok {
			quote, err = s.openDIYQuote(user, input.Quote)
			if err != nil {
				return result, err
			}
			paid := *quote
			paid.Token = ""
			paid.ConfigHash = ""
			paid.Changed = true
			result.Billing = &paid
		}
	}
	result.Receipt, err = repo.SaveXNodeDIY(ctx, DIYDocument{UserID: user, Revision: input.SettingsRevision, Cipher: cipher, NativeChanged: whitelistChanged, Unchanged: !changed, Billing: quote, Result: &result}, input.RoutingRevision, plan, providerops.CreateInput{ActorUserID: user, OwnerUserID: user, Kind: DIYOperationKind, IdempotencyKey: key, RequestFingerprint: fingerprint, Items: items}, s.now())
	return result, err
}
