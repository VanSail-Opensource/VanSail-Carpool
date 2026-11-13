package emby

import (
	"context"
	"fmt"
	"time"
)

// restoreRemoteIdentity is reached only after the stored ID was authoritatively
// missing and its retained exact candidate resolved to an existing remote user.
func (s *Service) restoreRemoteIdentity(ctx context.Context, expected ProvisioningRecord, user RemoteUser) (RemoteUser, error) {
	if user.ID == "" || user.Name != expected.CandidateUsername {
		return RemoteUser{}, fmt.Errorf("%w: restored identity does not match candidate", ErrAccountExists)
	}
	if user.ID != expected.RemoteUserID {
		if writer, ok := s.repository.(interface {
			RestoreEmbyRemoteIdentity(context.Context, ProvisioningRecord, string, string, time.Time) error
		}); ok {
			if err := writer.RestoreEmbyRemoteIdentity(ctx, expected, user.ID, user.Name, s.now().UTC()); err != nil {
				return RemoteUser{}, fmt.Errorf("restore Emby remote identity: %w", err)
			}
		}
	}
	return user, nil
}
