package xnode

import "context"

// CloseAccountPeriods uses existing revocation and journal barriers for every
// paid/PAYG principal. Offline participants keep cleanup pending, never skipped.
func (s *Service) CloseAccountPeriods(ctx context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	identities, err := s.repo.XNodeIdentities(ctx)
	if err != nil {
		return err
	}
	for _, identity := range identities {
		if identity.MemberOwnerID() != userID {
			continue
		}
		periods, err := s.repo.XNodePeriods(ctx, identity.ProxyUserID)
		if err != nil {
			return err
		}
		for _, period := range periods {
			if err = s.closePeriod(ctx, period); err != nil {
				return err
			}
		}
	}
	return nil
}
