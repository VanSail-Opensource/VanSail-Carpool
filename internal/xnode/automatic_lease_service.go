package xnode

import "context"

// AutomaticFirewallLeases reads volatile IP state and durable node configuration.
// The node list is read once; no lease IP, observation or delivery cursor is SQL.
func (s *Service) AutomaticFirewallLeases(ctx context.Context, search string, page, size int) (AutomaticLeasePage, error) {
	if !validAutomaticLeasePage(search, page, size) {
		return AutomaticLeasePage{}, ErrInvalid
	}
	if s.master == nil {
		return AutomaticLeasePage{}, ErrUnavailable
	}
	nodes, err := s.repo.XNodeNodes(ctx)
	if err != nil {
		return AutomaticLeasePage{}, err
	}
	var watermarks []uint64
	for _, node := range nodes {
		var settings NodeSettings
		if err := s.open("xnode.node."+node.ID, node.SettingsCipher, &settings); err != nil {
			return AutomaticLeasePage{}, err
		}
		if !settings.Firewall.Enabled {
			continue
		}
		s.master.mu.RLock()
		peer := s.master.peers[node.ID]
		var cursor uint64
		if peer != nil {
			peer.mu.Lock()
			cursor = peer.automaticCursor
			peer.mu.Unlock()
		}
		s.master.mu.RUnlock()
		watermarks = append(watermarks, cursor)
	}
	return s.master.automaticLeases.page(search, page, size, watermarks, s.Now())
}
