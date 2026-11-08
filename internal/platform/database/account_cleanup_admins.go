package database

import "errors"

type accountCleanupAdminSet struct{ ids map[int64]struct{} }

// ConfigureAccountCleanupAdmins installs the configured protection set before
// workers start. An uninitialized set disables cleanup, never authentication.
func (s *Store) ConfigureAccountCleanupAdmins(ids []int64) error {
	if len(ids) > 1024 {
		return errors.New("too many account cleanup administrators")
	}
	next := &accountCleanupAdminSet{ids: make(map[int64]struct{}, len(ids))}
	for _, id := range ids {
		if id <= 0 || id >= 1<<52 {
			return ErrConflict
		}
		next.ids[id] = struct{}{}
	}
	s.cleanupAdmins.Store(next)
	return nil
}

func (s *Store) cleanupConfiguredAdmin(id int64) bool {
	set := s.cleanupAdmins.Load()
	if set == nil {
		return false
	}
	_, ok := set.ids[id]
	return ok
}

func (s *Store) cleanupProtected(id int64, role string) bool {
	return s.cleanupAdmins.Load() == nil || role == "admin" || s.cleanupConfiguredAdmin(id)
}
