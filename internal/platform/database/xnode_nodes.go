package database

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

const xnodeNodeSelect = `SELECT id,name,country_code,provider_name,multiplier_fp,enabled,config_id,settings_cipher,token_hash,COALESCE(agent_id,''),desired_revision,accepted_revision,applied_revision,created_at,updated_at,retired_at,removing_at FROM xnode_nodes`

const saveXNodeNodeSQL = `INSERT INTO xnode_nodes(id,name,country_code,provider_name,multiplier_fp,enabled,config_id,settings_cipher,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(id) DO UPDATE SET name=excluded.name,country_code=excluded.country_code,provider_name=excluded.provider_name,multiplier_fp=excluded.multiplier_fp,enabled=excluded.enabled,config_id=excluded.config_id,settings_cipher=excluded.settings_cipher,updated_at=excluded.updated_at WHERE xnode_nodes.retired_at IS NULL AND xnode_nodes.removing_at IS NULL`

func xnodeNodeArgs(node xnode.Node) []any {
	return []any{node.ID, node.Name, node.CountryCode, node.ProviderName, xnode.MultiplierScale, boolInt(node.Enabled), node.ConfigID, node.SettingsCipher, stamp(time.Now().UTC()), stamp(time.Now().UTC())}
}

func scanXNodeNode(row rowScanner) (xnode.Node, error) {
	var node xnode.Node
	var enabled int
	var created, updated string
	var retired, removing sql.NullString
	err := row.Scan(&node.ID, &node.Name, &node.CountryCode, &node.ProviderName, &node.MultiplierFP, &enabled, &node.ConfigID, &node.SettingsCipher, &node.TokenHash, &node.AgentID, &node.DesiredRevision, &node.AcceptedRevision, &node.AppliedRevision, &created, &updated, &retired, &removing)
	if errors.Is(err, sql.ErrNoRows) {
		return node, xnode.ErrNotFound
	}
	if err != nil {
		return node, err
	}
	if removing.Valid {
		at, e := time.Parse(time.RFC3339Nano, removing.String)
		if e != nil {
			return node, e
		}
		node.RemovingAt = &at
	}
	if retired.Valid {
		at, e := time.Parse(time.RFC3339Nano, retired.String)
		if e != nil {
			return node, e
		}
		node.RetiredAt = &at
	}
	node.Enabled, node.Multiplier, node.MultiplierFP = enabled == 1, 1, xnode.MultiplierScale
	node.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return node, err
	}
	node.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated)
	return node, err
}

func (s *Store) XNodeNodes(ctx context.Context) ([]xnode.Node, error) {
	rows, err := s.db.QueryContext(ctx, xnodeNodeSelect+` WHERE retired_at IS NULL ORDER BY name,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []xnode.Node{}
	for rows.Next() {
		node, err := scanXNodeNode(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, node)
	}
	return out, rows.Err()
}
func (s *Store) XNodeNode(ctx context.Context, id string) (xnode.Node, error) {
	return scanXNodeNode(s.db.QueryRowContext(ctx, xnodeNodeSelect+` WHERE id=$1`, id))
}
func (s *Store) XNodeNodeByToken(ctx context.Context, hash []byte) (xnode.Node, error) {
	return scanXNodeNode(s.db.QueryRowContext(ctx, xnodeNodeSelect+` WHERE token_hash=$1 AND retired_at IS NULL`, hash))
}

func (s *Store) SaveXNodeNode(ctx context.Context, node xnode.Node) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	result, err := s.db.ExecContext(ctx, saveXNodeNodeSQL, xnodeNodeArgs(node)...)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count != 1 {
		return xnode.ErrConflict
	}
	return err
}
func (s *Store) DeleteXNodeNode(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM xnode_nodes WHERE id=$1 AND enabled=0`, id)
	return err
}
func (s *Store) RotateXNodeToken(ctx context.Context, id string, hash []byte) error {
	result, err := s.db.ExecContext(ctx, `UPDATE xnode_nodes SET token_hash=$1,updated_at=$2 WHERE id=$3 AND retired_at IS NULL AND removing_at IS NULL`, hash, stamp(time.Now().UTC()), id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count != 1 {
		return xnode.ErrConflict
	}
	return err
}
func (s *Store) retryAttempt_BindXNodeAgent(ctx context.Context, id, agent string) error {

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE xnode_nodes SET agent_id=$1 WHERE id=$2 AND retired_at IS NULL AND (agent_id IS NULL OR agent_id=$3)`, agent, id, agent)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return xnode.ErrConflict
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO xnode_node_rates(agent_id,from_sequence,multiplier_fp) VALUES($1,1,$2) ON CONFLICT DO NOTHING`, agent, xnode.MultiplierScale); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) AdvanceXNodeRevision(ctx context.Context, id string, observed uint64) (uint64, error) {
	if observed >= math.MaxInt64 {
		return 0, xnode.ErrConflict
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	var revision uint64
	err := s.db.QueryRowContext(ctx, `UPDATE xnode_nodes SET desired_revision=GREATEST(desired_revision,$1)+1 WHERE id=$2 AND desired_revision<$3 RETURNING desired_revision`, observed, id, int64(math.MaxInt64)).Scan(&revision)
	return revision, err
}
func (s *Store) AcknowledgeXNodeState(ctx context.Context, id string, revision uint64, applied bool) error {
	column := "accepted_revision"
	if applied {
		column = "applied_revision"
	}
	_, err := s.db.ExecContext(ctx, `UPDATE xnode_nodes SET `+column+`=GREATEST(`+column+`,$1) WHERE id=$2 AND desired_revision>=$3`, revision, id, revision)
	return err
}
