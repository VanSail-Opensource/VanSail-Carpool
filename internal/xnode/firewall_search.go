package xnode

import (
	"context"
	"net/netip"
	"strings"
)

type FirewallBanPage struct {
	Items      []FirewallBanView `json:"items"`
	NextCursor string            `json:"nextCursor,omitempty"`
}

func (s *Service) SearchFirewallBans(ctx context.Context, query, cursor string, limit int) (FirewallBanPage, error) {
	var prefix netip.Prefix
	if strings.TrimSpace(query) != "" {
		var err error
		prefix, err = ExemptionPrefix(query)
		if err != nil {
			return FirewallBanPage{}, err
		}
	}
	if limit < 1 || limit > 100 || len(cursor) > 512 {
		return FirewallBanPage{}, ErrInvalid
	}
	repo, ok := s.repo.(interface {
		SearchFirewallBans(context.Context, netip.Prefix, string, int) (FirewallBanPage, error)
	})
	if !ok {
		return FirewallBanPage{}, ErrUnavailable
	}
	return repo.SearchFirewallBans(ctx, prefix, cursor, limit)
}
