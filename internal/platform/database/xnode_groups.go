package database

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (s *Store) XNodeGroups(ctx context.Context) ([]xnode.Group, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,enabled FROM xnode_groups ORDER BY name,id`)
	if err != nil {
		return nil, err
	}
	out := []xnode.Group{}
	positions := map[string]int{}
	for rows.Next() {
		var item xnode.Group
		var enabled int
		if err = rows.Scan(&item.ID, &item.Name, &enabled); err != nil {
			rows.Close()
			return nil, err
		}
		item.Enabled = enabled == 1
		item.Bindings = []xnode.GroupBinding{}
		positions[item.ID] = len(out)
		out = append(out, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	bindings, err := s.db.QueryContext(ctx, `SELECT group_id,node_id,inbound_tag FROM xnode_group_bindings ORDER BY group_id,node_id,inbound_tag`)
	if err != nil {
		return nil, err
	}
	defer bindings.Close()
	for bindings.Next() {
		var item xnode.GroupBinding
		if err = bindings.Scan(&item.GroupID, &item.NodeID, &item.InboundTag); err != nil {
			return nil, err
		}
		position, ok := positions[item.GroupID]
		if ok {
			out[position].Bindings = append(out[position].Bindings, item)
		}
	}
	return out, bindings.Err()
}
func (s *Store) SaveXNodeGroup(ctx context.Context, item xnode.Group) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO xnode_groups(id,name,enabled,created_at,updated_at) VALUES(?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name,enabled=excluded.enabled,updated_at=excluded.updated_at`, item.ID, item.Name, boolInt(item.Enabled), stamp(time.Now().UTC()), stamp(time.Now().UTC()))
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM xnode_group_bindings WHERE group_id=?`, item.ID); err != nil {
		return err
	}
	for _, binding := range item.Bindings {
		if _, err = tx.ExecContext(ctx, `INSERT INTO xnode_group_bindings(group_id,node_id,inbound_tag) VALUES(?,?,?)`, item.ID, binding.NodeID, binding.InboundTag); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Store) DeleteXNodeGroup(ctx context.Context, id string) error {
	var used int
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM combos,json_each(combos.included_squad_uuids) selected WHERE selected.value=? UNION SELECT 1 FROM purchase_squads WHERE remna_squad_uuid=?)`, id, id).Scan(&used)
	if err != nil {
		return err
	}
	if used != 0 {
		return xnode.ErrConflict
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM xnode_groups WHERE id=?`, id)
	return err
}
