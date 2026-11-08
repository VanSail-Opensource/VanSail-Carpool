package xnode

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// Hourly authorization uses the committed paid boundary directly. No short PAYG
// lease or legacy continuity allowance may extend it beyond purchased access.
func (s *Service) hourlyUsers(ctx context.Context, node Node, inbounds map[string]map[string]any) ([]wire.User, error) {
	result := []wire.User{}
	if node.RemovingAt != nil || node.RetiredAt != nil || s.master == nil || !s.master.HourlyCapable(node.ID) {
		return result, nil
	}
	principals, err := s.hourlyPrincipals(ctx, "", node.ID)
	if err != nil || len(principals) == 0 {
		return result, err
	}
	groups, err := s.repo.XNodeGroups(ctx)
	if err != nil {
		return nil, err
	}
	byGroup := map[string]Group{}
	for _, group := range groups {
		byGroup[group.ID] = group
	}
	now := s.now().UTC()
	for _, p := range principals {
		preview := p.Status == "preparing" && p.EnrollmentID == s.hourlyPreview
		if preview {
			p.Status, p.PaidThrough = "active", now.Add(time.Minute)
		}
		group := byGroup[p.GroupID]
		if !hourlyPrincipalAvailable(p, now) || !group.Enabled {
			continue
		}
		owner, e := s.repo.UserByID(ctx, p.OwnerID)
		if e != nil {
			return nil, e
		}
		parent, e := s.repo.XNodeIdentityByUser(ctx, p.OwnerID)
		if e != nil {
			return nil, e
		}
		restricted, e := s.repo.XNodeUserRestricted(ctx, p.OwnerID)
		if e != nil {
			return nil, e
		}
		if owner.AccountGeneration != p.AccountGeneration || owner.AccountStatus != "" && owner.AccountStatus != "active" || owner.OnboardingState != "complete" || parent.AccessDisabled || restricted {
			continue
		}
		identity, e := s.repo.XNodeIdentity(ctx, p.ProxyUserID)
		if e != nil {
			return nil, e
		}
		if identity.AccessDisabled || identity.HourlyOwnerID != p.OwnerID || identity.CurrentPeriodID != p.PeriodID || identity.ServiceName != hourlyPrincipalPrefix+p.EnrollmentID {
			continue
		}
		period, e := s.repo.XNodePeriod(ctx, p.PeriodID)
		if e != nil {
			return nil, e
		}
		if period.Status != "open" || period.ProxyUserID != p.ProxyUserID || period.Strategy != "NO_RESET" || period.PurchaseID != "" {
			continue
		}
		credentials, e := loadCredentials(s.vault, identity)
		if e != nil {
			return nil, e
		}
		until := p.PaidThrough.UTC()
		user := wire.User{ID: int(p.ProxyUserID), UUID: credentials.UUID, AccountingPeriod: p.PeriodID, ValidUntil: &until, Hourly: &wire.HourlyUser{Owner: wire.HourlyOwner{ID: p.OwnerID, Generation: uint64(p.AccountGeneration)}, EnrollmentID: p.EnrollmentID}, Credentials: map[string]json.RawMessage{}}
		user.BlockNew = preview
		seen := map[string]bool{}
		for _, binding := range group.Bindings {
			if binding.NodeID != node.ID || seen[binding.InboundTag] {
				continue
			}
			if inbounds[binding.InboundTag] == nil {
				return nil, ErrInvalid
			}
			seen[binding.InboundTag] = true
			credential, e := inboundCredential(inbounds[binding.InboundTag], credentials)
			if e != nil {
				return nil, e
			}
			user.InboundTags = append(user.InboundTags, binding.InboundTag)
			user.Credentials[binding.InboundTag] = credential
		}
		sort.Strings(user.InboundTags)
		if len(user.InboundTags) > 0 {
			result = append(result, user)
		}
	}
	return result, nil
}
