package database

import (
	"context"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/abuse"
)

// ReconcileNodeCredentials removes secrets only after a complete successful inventory.
// Historical detector facts and incidents are preserved.
func (s *Store) ReconcileNodeCredentials(ctx context.Context, nodes []abuse.Node) error {
	uuids := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if node.UUID == "" {
			return abuse.ErrInvalid
		}
		uuids = append(uuids, node.UUID)
	}
	encoded, err := json.Marshal(uuids)
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM abuse_node_credentials WHERE node_uuid NOT IN (SELECT value FROM json_each(?))`, string(encoded)); err != nil {
		return err
	}
	for _, node := range nodes {
		if _, err := tx.ExecContext(ctx, `UPDATE abuse_node_credentials SET node_name=?,updated_at=? WHERE node_uuid=?`, node.Name, stamp(time.Now().UTC()), node.UUID); err != nil {
			return err
		}
	}
	return tx.Commit()
}
