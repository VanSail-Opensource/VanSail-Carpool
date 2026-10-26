package database

import "context"

func (s *Store) HostNameOverrides(ctx context.Context, user string) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT host_id,name FROM user_host_names WHERE user_id=?`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, name string
		if err = rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = name
	}
	return out, rows.Err()
}

func (s *Store) SaveHostNameOverride(ctx context.Context, user, host, name string) error {
	if name == "" {
		_, err := s.db.ExecContext(ctx, `DELETE FROM user_host_names WHERE user_id=? AND host_id=?`, user, host)
		return err
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO user_host_names(user_id,host_id,name) VALUES(?,?,?) ON CONFLICT(user_id,host_id) DO UPDATE SET name=excluded.name`, user, host, name)
	return err
}
