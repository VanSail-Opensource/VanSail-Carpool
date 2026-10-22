package xnode

import (
	"context"
	"encoding/json"
	"slices"
	"sort"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (s *Service) eligibleTags(ctx context.Context, userID, nodeID string) ([]string, *Period, error) {
	access, err := s.memberAccess(ctx, userID)
	if err != nil || !access.allowed {
		return []string{}, access.period, err
	}
	purchase := access.purchase
	disabled, err := s.repo.UserDisabledSquads(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	groups, err := s.repo.XNodeGroups(ctx)
	if err != nil {
		return nil, nil, err
	}
	tags := map[string]bool{}
	for _, group := range groups {
		if !group.Enabled || !slices.Contains(purchase.SquadUUIDs, group.ID) || slices.Contains(disabled, group.ID) {
			continue
		}
		for _, binding := range group.Bindings {
			if binding.NodeID == nodeID {
				tags[binding.InboundTag] = true
			}
		}
	}
	out := []string{}
	for tag := range tags {
		out = append(out, tag)
	}
	sort.Strings(out)
	return out, access.period, nil
}

func (s *Service) accessibleNodeIDs(ctx context.Context, userID string) (map[string]bool, error) {
	nodes, err := s.repo.XNodeNodes(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, node := range nodes {
		if !node.Enabled {
			continue
		}
		tags, _, err := s.eligibleTags(ctx, userID, node.ID)
		if err != nil {
			return nil, err
		}
		if len(tags) > 0 {
			out[node.ID] = true
		}
	}
	return out, nil
}

func (s *Service) nodeUsers(ctx context.Context, node Node, native map[string]any, global GlobalConfig) ([]wire.User, error) {
	users := []wire.User{}
	if !node.Enabled {
		return users, nil
	}
	state, err := s.repo.XNodeMigration(ctx)
	if err != nil {
		return nil, err
	}
	for _, migration := range state.Nodes {
		if migration.NodeID == node.ID && migration.Phase != "native" {
			return users, nil
		}
	}
	identities, err := s.repo.XNodeIdentities(ctx)
	if err != nil {
		return nil, err
	}
	inbounds, ok := native["inbounds"].([]any)
	if !ok {
		return nil, ErrInvalid
	}
	byTag := map[string]map[string]any{}
	for _, item := range inbounds {
		inbound, ok := item.(map[string]any)
		if !ok {
			return nil, ErrInvalid
		}
		tag, _ := inbound["tag"].(string)
		byTag[tag] = inbound
	}
	for _, identity := range identities {
		if identity.UserID == "" {
			user, err := s.serviceUser(ctx, identity, node, byTag)
			if err != nil {
				return nil, err
			}
			if len(user.InboundTags) > 0 {
				users = append(users, user)
			}
			continue
		}
		tags, period, err := s.eligibleTags(ctx, identity.UserID, node.ID)
		if err != nil {
			return nil, err
		}
		if len(tags) == 0 {
			continue
		}
		purchase, err := s.EffectivePurchase(ctx, identity.UserID)
		if err != nil || purchase == nil {
			return nil, err
		}
		credentials, err := loadCredentials(s.vault, identity)
		if err != nil {
			return nil, err
		}
		until, err := s.continuityUntil(ctx, *purchase)
		if err != nil {
			return nil, err
		}
		user := wire.User{ID: int(identity.ProxyUserID), UUID: credentials.UUID, InboundTags: tags, AccountingPeriod: period.ID, ValidUntil: &until, Credentials: map[string]json.RawMessage{}}
		if s.master != nil {
			user.BlockNew = s.master.Blocked(user.ID)
		}
		for _, tag := range tags {
			inbound := byTag[tag]
			if inbound == nil {
				return nil, ErrInvalid
			}
			credential, err := inboundCredential(inbound, credentials)
			if err != nil {
				return nil, err
			}
			user.Credentials[tag] = credential
		}
		users = append(users, user)
	}
	return users, nil
}
