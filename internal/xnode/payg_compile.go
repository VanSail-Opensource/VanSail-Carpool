package xnode

import (
	"context"
	"encoding/json"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"sort"
)

// Financial eligibility and short leases are overlaid at synchronization, not
// compiled into native Xray documents or persisted as member preferences.
func (s *Service) paygUsers(ctx context.Context, node Node, inbounds map[string]map[string]any) ([]wire.User, error) {
	out := []wire.User{}
	r, ok := s.paygRepository()
	if !ok {
		return out, nil
	}
	accounts, err := r.XNodePAYGAccounts(ctx, "", node.ID)
	if err != nil {
		return nil, err
	}
	products, err := r.ListSquadProducts(ctx, false)
	if err != nil {
		return nil, err
	}
	payg := map[string]bool{}
	for _, p := range products {
		payg[p.GroupID] = p.BillingMode == "payg"
	}
	groups, err := s.repo.XNodeGroups(ctx)
	if err != nil {
		return nil, err
	}
	byGroup := map[string]Group{}
	for _, g := range groups {
		byGroup[g.ID] = g
	}
	coverage := map[string]map[string]string{}
	allowed := map[string]bool{}
	for _, a := range accounts {
		if _, seen := coverage[a.UserID]; !seen {
			coverage[a.UserID], err = s.paygCoverage(ctx, a.UserID)
			if err != nil {
				return nil, err
			}
			user, e := s.repo.UserByID(ctx, a.UserID)
			if e != nil {
				return nil, e
			}
			parent, e := s.repo.XNodeIdentityByUser(ctx, a.UserID)
			if e != nil {
				return nil, e
			}
			restricted, e := s.repo.XNodeUserRestricted(ctx, a.UserID)
			if e != nil {
				return nil, e
			}
			allowed[a.UserID] = user.OnboardingState == "complete" && !parent.AccessDisabled && !restricted
		}
		group := byGroup[a.GroupID]
		if !payg[a.GroupID] || !group.Enabled || !allowed[a.UserID] || coverage[a.UserID][a.GroupID] != "" {
			continue
		}
		i, e := s.repo.XNodeIdentity(ctx, a.ProxyUserID)
		if e != nil {
			return nil, e
		}
		if i.AccessDisabled {
			continue
		}
		c, e := loadCredentials(s.vault, i)
		if e != nil {
			return nil, e
		}
		u := wire.User{ID: int(i.ProxyUserID), UUID: c.UUID, AccountingPeriod: a.PeriodID, InboundTags: []string{}, Credentials: map[string]json.RawMessage{}}
		seen := map[string]bool{}
		for _, b := range group.Bindings {
			if b.NodeID != node.ID || seen[b.InboundTag] {
				continue
			}
			seen[b.InboundTag] = true
			inbound := inbounds[b.InboundTag]
			if inbound == nil {
				continue
			}
			credential, e := inboundCredential(inbound, c)
			if e != nil {
				return nil, e
			}
			u.InboundTags = append(u.InboundTags, b.InboundTag)
			u.Credentials[b.InboundTag] = credential
		}
		sort.Strings(u.InboundTags)
		if err = s.applyMemberWhitelist(ctx, a.UserID, &u); err != nil {
			return nil, err
		}
		if len(u.InboundTags) > 0 {
			out = append(out, u)
		}
	}
	return out, nil
}

func (s *Service) extendOwnerRules(ctx context.Context, users []wire.User, custom Customization, rules []any) error {
	for _, raw := range rules {
		rule, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		selectors := []string{}
		for _, user := range users {
			identity, e := s.repo.XNodeIdentity(ctx, int64(user.ID))
			if e != nil {
				return e
			}
			if identity.MemberOwnerID() == custom.UserID && custom.UserID != "" {
				selectors = append(selectors, userMatcher(int64(user.ID)))
			}
		}
		if len(selectors) > 0 {
			rule["user"] = selectors
		}
	}
	return nil
}
