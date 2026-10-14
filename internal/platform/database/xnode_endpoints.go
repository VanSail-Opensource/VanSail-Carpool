package database

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (s *Store) XNodeEndpoints(ctx context.Context) ([]xnode.Endpoint, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,node_id,inbound_tag,name,address,port,enabled,hidden,overrides_cipher FROM xnode_endpoints ORDER BY name,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []xnode.Endpoint{}
	for rows.Next() {
		var item xnode.Endpoint
		var enabled, hidden int
		if err = rows.Scan(&item.ID, &item.NodeID, &item.InboundTag, &item.Name, &item.Address, &item.Port, &enabled, &hidden, &item.OverridesCipher); err != nil {
			return nil, err
		}
		item.Enabled, item.Hidden = enabled == 1, hidden == 1
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *Store) SaveXNodeEndpoint(ctx context.Context, item xnode.Endpoint) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO xnode_endpoints(id,node_id,inbound_tag,name,address,port,enabled,hidden,overrides_cipher,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET node_id=excluded.node_id,inbound_tag=excluded.inbound_tag,name=excluded.name,address=excluded.address,port=excluded.port,enabled=excluded.enabled,hidden=excluded.hidden,overrides_cipher=excluded.overrides_cipher,updated_at=excluded.updated_at`, item.ID, item.NodeID, item.InboundTag, item.Name, item.Address, item.Port, boolInt(item.Enabled), boolInt(item.Hidden), item.OverridesCipher, stamp(time.Now().UTC()), stamp(time.Now().UTC()))
	return err
}
func (s *Store) DeleteXNodeEndpoint(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM xnode_endpoints WHERE id=?`, id)
	return err
}
