package database

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"time"
)

func (s *Store) XNodeServiceBindings(ctx context.Context, id int64) ([]xnode.ServiceBinding, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT proxy_user_id,node_id,inbound_tag,valid_until FROM xnode_service_bindings WHERE proxy_user_id=? ORDER BY node_id,inbound_tag`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []xnode.ServiceBinding{}
	for rows.Next() {
		var item xnode.ServiceBinding
		var until string
		if err = rows.Scan(&item.ProxyUserID, &item.NodeID, &item.InboundTag, &until); err != nil {
			return nil, err
		}
		item.ValidUntil, err = time.Parse(time.RFC3339Nano, until)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *Store) SaveXNodeServiceBinding(ctx context.Context, item xnode.ServiceBinding) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO xnode_service_bindings(proxy_user_id,node_id,inbound_tag,valid_until) SELECT ?,?,?,? WHERE EXISTS(SELECT 1 FROM xnode_identities WHERE proxy_user_id=? AND service_name IS NOT NULL) ON CONFLICT DO UPDATE SET valid_until=excluded.valid_until`, item.ProxyUserID, item.NodeID, item.InboundTag, stamp(item.ValidUntil), item.ProxyUserID)
	return err
}
