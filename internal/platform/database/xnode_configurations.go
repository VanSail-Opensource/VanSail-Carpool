package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func scanXNodeConfiguration(row rowScanner) (xnode.Configuration, error) {
	var config xnode.Configuration
	var created, updated string
	err := row.Scan(&config.ID, &config.Name, &config.DocumentCipher, &config.Revision, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return config, xnode.ErrNotFound
	}
	if err != nil {
		return config, err
	}
	config.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return config, err
	}
	config.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated)
	config.SnippetIDs = []string{}
	return config, err
}
func (s *Store) XNodeConfigurations(ctx context.Context) ([]xnode.Configuration, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,document_cipher,revision,created_at,updated_at FROM xnode_configs ORDER BY name,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []xnode.Configuration{}
	for rows.Next() {
		config, err := scanXNodeConfiguration(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, config)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	for i := range out {
		out[i], err = s.XNodeConfiguration(ctx, out[i].ID)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
func (s *Store) XNodeConfiguration(ctx context.Context, id string) (xnode.Configuration, error) {
	config, err := scanXNodeConfiguration(s.db.QueryRowContext(ctx, `SELECT id,name,document_cipher,revision,created_at,updated_at FROM xnode_configs WHERE id=?`, id))
	if err != nil {
		return config, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT snippet_id FROM xnode_config_snippets WHERE config_id=? ORDER BY position`, id)
	if err != nil {
		return config, err
	}
	defer rows.Close()
	for rows.Next() {
		var snippet string
		if err = rows.Scan(&snippet); err != nil {
			return config, err
		}
		config.SnippetIDs = append(config.SnippetIDs, snippet)
	}
	return config, rows.Err()
}
func (s *Store) retryAttempt_SaveXNodeConfiguration(ctx context.Context, config xnode.Configuration) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if config.Revision == 0 {
		_, err = tx.ExecContext(ctx, `INSERT INTO xnode_configs(id,name,document_cipher,revision,created_at,updated_at) VALUES(?,?,?,1,?,?)`, config.ID, config.Name, config.DocumentCipher, stamp(time.Now().UTC()), stamp(time.Now().UTC()))
	} else {
		var result sql.Result
		result, err = tx.ExecContext(ctx, `UPDATE xnode_configs SET name=?,document_cipher=?,revision=revision+1,updated_at=? WHERE id=? AND revision=?`, config.Name, config.DocumentCipher, stamp(time.Now().UTC()), config.ID, config.Revision)
		if err == nil {
			count, countErr := result.RowsAffected()
			if countErr != nil {
				return countErr
			}
			if count != 1 {
				return xnode.ErrConflict
			}
		}
	}
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM xnode_config_snippets WHERE config_id=?`, config.ID); err != nil {
		return err
	}
	for position, id := range config.SnippetIDs {
		if _, err = tx.ExecContext(ctx, `INSERT INTO xnode_config_snippets(config_id,snippet_id,position) VALUES(?,?,?)`, config.ID, id, position); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Store) DeleteXNodeConfiguration(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM xnode_configs WHERE id=?`, id)
	return err
}
