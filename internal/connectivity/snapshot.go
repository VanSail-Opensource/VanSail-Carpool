package connectivity

import "context"

// Snapshot derives latest attempts from retained history and transient metadata.
// A source outage preserves the last successful inventory and marks it stale.
func (s *Service) Snapshot(ctx context.Context) (Snapshot, error) {
	s.mu.Lock()
	cfg, err := s.readConfig(ctx)
	if err != nil {
		s.mu.Unlock()
		return Snapshot{}, err
	}
	hash := ConfigHash(cfg)
	s.applyConfigLocked(cfg)
	s.mu.Unlock()
	result := Snapshot{Config: cfg, Hosts: []HostResult{}}
	result.Run, _, result.ErrorCode = s.runState(hash)
	if cfg.ProxyUserID == 0 {
		result.ErrorCode = "CONNECTIVITY_NOT_CONFIGURED"
		return result, nil
	}
	_, _, sourceErr := s.loadTargets(ctx, cfg, hash, false)
	user, targets, sourceCode := s.cacheState(hash)
	result.User = user
	if sourceErr != nil {
		result.Stale = true
		result.ErrorCode = ErrorCode(sourceErr)
	} else if sourceCode != "" {
		result.Stale, result.ErrorCode = true, sourceCode
	}
	items, err := s.repository.LatestConnectivityAttempts(ctx, hash, s.now().UTC())
	if err != nil {
		return Snapshot{}, &CodeError{Code: "CONNECTIVITY_STORAGE_UNAVAILABLE", Err: err}
	}
	latest := make(map[string]Attempt, len(items))
	for _, item := range items {
		latest[item.HostUUID] = item
	}
	for _, target := range targets {
		host := HostResult{HostUUID: target.HostUUID, Remark: target.Remark, Address: target.Address, Port: target.Port}
		if attempt, exists := latest[target.HostUUID]; exists {
			copy := attempt
			host.Latest = &copy
		}
		result.Hosts = append(result.Hosts, host)
		delete(latest, target.HostUUID)
	}
	if sourceErr != nil {
		for _, item := range items {
			if _, exists := latest[item.HostUUID]; item.HostUUID != "" && exists {
				copy := item
				result.Hosts = append(result.Hosts, HostResult{HostUUID: item.HostUUID, Latest: &copy})
				delete(latest, item.HostUUID)
			}
		}
	}
	var ready bool
	var storageCode string
	result.Run, ready, storageCode = s.runState(hash)
	if storageCode != "" {
		result.Stale, result.ErrorCode = true, storageCode
	}
	if !ready {
		result.Stale = true
	}
	return result, nil
}
