package database

import "context"

func (s *Store) XNodeUserRestricted(ctx context.Context, userID string) (bool, error) {
	var restricted bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM admin_temporary_bans WHERE user_id=$1 AND restored_at IS NULL) OR EXISTS(SELECT 1 FROM xnode_identities WHERE user_id=$2 AND access_disabled=1) OR EXISTS(SELECT 1 FROM users WHERE id=$3 AND account_status<>'active')`, userID, userID, userID).Scan(&restricted)
	return restricted, err
}

func (s *Store) SetXNodeAccessDisabled(ctx context.Context, id int64, disabled bool) error {
	_, err := s.db.ExecContext(ctx, `UPDATE xnode_identities SET access_disabled=$1 WHERE proxy_user_id=$2 AND ($3=1 OR NOT EXISTS(SELECT 1 FROM users WHERE users.id=xnode_identities.user_id AND account_status<>'active'))`, boolInt(disabled), id, boolInt(disabled))
	return err
}
