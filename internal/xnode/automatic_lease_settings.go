package xnode

import "context"

// Merge only the lease setting under the existing global writer lock. A lease
// form loaded before another administrator's change cannot replace credentials,
// report intervals or any other unrelated desired state.
func (s *Service) SaveAutomaticWhitelist(ctx context.Context, actor string, settings AutomaticWhitelistConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, err := s.Global(ctx)
	if err != nil {
		return err
	}
	next := cloneGlobal(previous)
	next.AutomaticWhitelist = &settings
	next, err = prepareGlobalChange(previous, next)
	if err != nil {
		return err
	}
	fence, err := s.captureGlobalWrite(ctx)
	if err != nil {
		return err
	}
	return s.commitGlobalLocked(ctx, actor, previous, next, fence)
}
