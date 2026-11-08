package xnode

import (
	"context"
	"slices"
	"strconv"
	"strings"
)

func customizationOwner(item Customization) string {
	if item.UserID != "" {
		return item.UserID
	}
	return "native." + strconv.FormatInt(item.ProxyUserID, 10)
}
func identityCustomizationOwner(item Identity) string {
	if item.MemberOwnerID() != "" {
		return item.MemberOwnerID()
	}
	return customizationOwner(Customization{UserID: item.UserID, ProxyUserID: item.ProxyUserID})
}
func (s *Service) customizationIdentity(ctx context.Context, owner, node string) (Identity, []string, error) {
	if !strings.HasPrefix(owner, "native.") {
		identity, err := s.repo.XNodeIdentityByUser(ctx, owner)
		if err != nil {
			return identity, nil, err
		}
		tags, _, err := s.eligibleTags(ctx, owner, node)
		if err == nil {
			hourly, e := s.hourlyMemberTags(ctx, owner, node)
			if e != nil {
				return identity, nil, e
			}
			tags = append(tags, hourly...)
			slices.Sort(tags)
			tags = slices.Compact(tags)
		}
		return identity, tags, err
	}
	identity, err := s.Identity(ctx, strings.TrimPrefix(owner, "native."))
	if err != nil {
		return identity, nil, err
	}
	access, err := s.ServiceAccess(ctx, identity)
	if err != nil {
		return identity, nil, err
	}
	if access == nil {
		return identity, nil, ErrNotFound
	}
	bindings, err := s.serviceBindings(ctx, identity, access)
	var tags []string
	for _, binding := range bindings {
		if binding.NodeID == node {
			tags = append(tags, binding.InboundTag)
		}
	}
	return identity, tags, err
}
