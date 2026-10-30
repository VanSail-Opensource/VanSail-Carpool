package xnode

import (
	"context"
	"slices"
	"time"
)

type groupDeletionRepository interface {
	XNodeGroupUsage(context.Context, time.Time) (map[string]GroupUsage, error)
	XNodeCompileVersion(context.Context, time.Time) (int64, time.Time, error)
	DeleteXNodeGroupAtVersion(context.Context, string, int64, []Endpoint) error
}

// AdminGroups enriches display reads without burdening compilation with counts.
func (s *Service) AdminGroups(ctx context.Context) ([]Group, error) {
	r, ok := s.repo.(groupDeletionRepository)
	if !ok {
		return nil, ErrUnavailable
	}
	groups, err := s.repo.XNodeGroups(ctx)
	if err != nil {
		return nil, err
	}
	usage, err := r.XNodeGroupUsage(ctx, s.now().UTC())
	if err != nil {
		return nil, err
	}
	services, err := s.NamedServices(ctx)
	if err != nil {
		return nil, err
	}
	for i := range groups {
		u := usage[groups[i].ID]
		groups[i].MemberCount = &u.Members
		groups[i].CanDelete = u.Members == 0 && !u.PendingAccounting && !groupHasService(services, groups[i].ID, s.now())
	}
	return groups, nil
}

func groupHasService(services []NamedService, id string, now time.Time) bool {
	for _, item := range services {
		if item.Access.ExpiresAt.After(now) && slices.Contains(item.Access.GroupIDs, id) {
			return true
		}
	}
	return false
}

func (s *Service) DeleteGroup(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.repo.(groupDeletionRepository)
	if !ok {
		return ErrUnavailable
	}
	version, _, err := r.XNodeCompileVersion(ctx, s.now().UTC())
	if err != nil {
		return err
	}
	services, err := s.NamedServices(ctx)
	if err != nil {
		return err
	}
	if groupHasService(services, id, s.now()) {
		return ErrConflict
	}
	endpoints, err := s.groupDeletionEndpoints(ctx, id)
	if err != nil {
		return err
	}
	// Any concurrent service or catalog change invalidates the encrypted-term read.
	return r.DeleteXNodeGroupAtVersion(ctx, id, version, endpoints)
}
