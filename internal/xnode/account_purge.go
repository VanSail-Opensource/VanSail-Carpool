package xnode

import (
	"context"
	"errors"
	"fmt"
)

// CloseAccountPeriods uses existing revocation and journal barriers for every
// paid/PAYG principal. Offline participants keep cleanup pending, never skipped.
func (s *Service) CloseAccountPeriods(ctx context.Context, userID string) error {
	identities, err := s.repo.XNodeIdentities(ctx)
	if err != nil {
		return err
	}
	var failures error
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
				failures = errors.Join(failures, fmt.Errorf("owner %s period %s: %w", userID, period.ID, err))
			}
		}
	}
	return failures
}
