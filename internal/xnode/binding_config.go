package xnode

import (
	"context"
	"encoding/json"
)

func (s *Service) validateManagedBinding(ctx context.Context, nodeID, tag string) error {
	node, err := s.repo.XNodeNode(ctx, nodeID)
	if err != nil {
		return err
	}
	profile, err := s.ResolveConfiguration(ctx, node.ConfigID)
	if err != nil {
		return err
	}
	var native struct {
		Inbounds []struct {
			Tag      string `json:"tag"`
			Protocol string `json:"protocol"`
		} `json:"inbounds"`
	}
	if json.Unmarshal(profile.Document, &native) != nil {
		return ErrInvalid
	}
	for _, inbound := range native.Inbounds {
		if inbound.Tag != tag {
			continue
		}
		switch inbound.Protocol {
		case "vless", "vmess", "trojan", "shadowsocks", "hysteria", "socks", "http", "mixed":
			return nil
		}
	}
	return ErrInvalid
}
