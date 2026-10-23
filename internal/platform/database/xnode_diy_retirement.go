package database

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (s *Store) XNodeDIYUsers(ctx context.Context, after string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT user_id FROM xnode_member_diy WHERE user_id>? ORDER BY user_id LIMIT 100`, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []string{}
	for rows.Next() {
		var user string
		if err = rows.Scan(&user); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (s *Store) ReplaceXNodeDIY(ctx context.Context, doc xnode.DIYDocument, cipher string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	result, err := s.db.ExecContext(ctx, `UPDATE xnode_member_diy SET revision=revision+1,cipher=? WHERE user_id=? AND revision=? AND cipher=? AND revision<9007199254740991`, cipher, doc.UserID, doc.Revision, doc.Cipher)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrConflict
	}
	return nil
}
