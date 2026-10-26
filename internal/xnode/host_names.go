package xnode

import (
	"context"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

type hostNameRepository interface {
	HostNameOverrides(context.Context, string) (map[string]string, error)
	SaveHostNameOverride(context.Context, string, string, string) error
}

type HostNameCustomization struct {
	HostID        string `json:"hostId"`
	Name          string `json:"name"`
	NameOverride  string `json:"nameOverride"`
	PublishedName string `json:"publishedName"`
	PriceLabel    string `json:"priceLabel"`
}

func (s *Service) hostNameRepository() (hostNameRepository, bool) {
	base := s.repo
	if reads, ok := base.(*compileReads); ok {
		base = reads.Repository
	}
	r, ok := base.(hostNameRepository)
	return r, ok
}

// SaveOwnedHostName authorizes the exact exported virtual or published host ID.
// It stores a display choice only; no native operation or paid setting is queued.
func (s *Service) SaveOwnedHostName(ctx context.Context, user, host, name string) (HostNameCustomization, error) {
	name = strings.TrimSpace(name)
	if host == "" || len(host) > 256 || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 100 || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return HostNameCustomization{}, ErrInvalid
	}
	r, ok := s.hostNameRepository()
	if !ok {
		return HostNameCustomization{}, ErrUnavailable
	}
	proxies, err := s.MemberProxies(ctx, user)
	if err != nil {
		return HostNameCustomization{}, err
	}
	proxies, err = s.ExportProxiesForUser(ctx, user, proxies)
	if err != nil {
		return HostNameCustomization{}, err
	}
	var selected *subscription.Proxy
	for i := range proxies {
		if proxies[i].ID == host {
			selected = &proxies[i]
			break
		}
	}
	if selected == nil {
		return HostNameCustomization{}, ErrNotFound
	}
	if err = r.SaveHostNameOverride(ctx, user, host, name); err != nil {
		return HostNameCustomization{}, err
	}
	template := name
	if template == "" {
		template = selected.PublishedName
	}
	return HostNameCustomization{HostID: host, Name: strings.ReplaceAll(template, "{price}", selected.PriceLabel), NameOverride: name, PublishedName: selected.PublishedName, PriceLabel: selected.PriceLabel}, nil
}

func (s *Service) presentHosts(ctx context.Context, user string, proxies []subscription.Proxy) ([]subscription.Proxy, error) {
	names := map[string]string{}
	if r, ok := s.hostNameRepository(); ok && user != "" {
		var err error
		names, err = r.HostNameOverrides(ctx, user)
		if err != nil {
			return nil, err
		}
	}
	labels, err := s.hostPriceLabels(ctx, user, proxies)
	if err != nil {
		return nil, err
	}
	result := append([]subscription.Proxy(nil), proxies...)
	for i := range result {
		p := &result[i]
		p.PublishedName = p.Name
		p.NameOverride = names[p.ID]
		p.PriceLabel = labels[p.ID]
		template := p.PublishedName
		if p.NameOverride != "" {
			template = p.NameOverride
		}
		p.ExportName = p.NameOverride != "" || strings.Contains(template, "{price}")
		p.Name = strings.ReplaceAll(template, "{price}", p.PriceLabel)
	}
	return result, nil
}
