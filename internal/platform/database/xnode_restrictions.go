package database

import "context"

func (s *Store) XNodeUserRestricted(ctx context.Context, userID string) (bool, error) {
	var restricted bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM admin_temporary_bans WHERE user_id=? AND restored_at IS NULL) OR EXISTS(SELECT 1 FROM xnode_identities WHERE user_id=? AND access_disabled=1)`, userID, userID).Scan(&restricted)
	return restricted, err
}

func (s *Store) SetXNodeAccessDisabled(ctx context.Context, id int64, disabled bool) error {
	_, err := s.db.ExecContext(ctx, `UPDATE xnode_identities SET access_disabled=? WHERE proxy_user_id=?`, boolInt(disabled), id)
	return err
}
