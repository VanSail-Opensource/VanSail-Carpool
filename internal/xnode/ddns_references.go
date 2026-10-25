package xnode

import (
	"context"
	"encoding/json"
)

// Literal endpoint and custom outbound references retain old bypass addresses.
// Managed routing generations additionally retain their old bank until every
// affected route passes its existing applied-state barriers.
func (s *Service) ddnsAddressReferences(ctx context.Context) (map[string]bool, error) {
	endpoints, err := s.repo.XNodeEndpoints(ctx)
	if err != nil {
		return nil, err
	}
	references := map[string]bool{}
	for _, host := range endpoints {
		if ip, e := FirewallPublicIP(host.Address); e == nil {
			references[ip] = true
		}
	}
	var customs []Customization
	if repo, ok := s.repo.(interface {
		XNodeAllCustomizations(context.Context) ([]Customization, error)
	}); ok {
		customs, err = repo.XNodeAllCustomizations(ctx)
	} else {
		nodes, e := s.Nodes(ctx)
		if e != nil {
			return nil, e
		}
		for _, node := range nodes {
			items, e := s.repo.XNodeCustomizations(ctx, node.ID)
			if e != nil {
				return nil, e
			}
			customs = append(customs, items...)
		}
	}
	if err != nil {
		return nil, err
	}
	for _, custom := range customs {
		if custom.OutboundsCipher == "" {
			continue
		}
		var outs json.RawMessage
		if err = s.open("xnode.outbounds."+customizationOwner(custom)+"."+custom.NodeID, custom.OutboundsCipher, &outs); err != nil {
			return nil, err
		}
		var values any
		if err = json.Unmarshal(outs, &values); err != nil {
			return nil, err
		}
		collectDDNSAddressReferences(values, references)
	}
	return references, nil
}

func collectDDNSAddressReferences(value any, references map[string]bool) {
	switch value := value.(type) {
	case string:
		if ip, err := FirewallPublicIP(value); err == nil {
			references[ip] = true
		}
	case []any:
		for _, item := range value {
			collectDDNSAddressReferences(item, references)
		}
	case map[string]any:
		for _, item := range value {
			collectDDNSAddressReferences(item, references)
		}
	}
}
