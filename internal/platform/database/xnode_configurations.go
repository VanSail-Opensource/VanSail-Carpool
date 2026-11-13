package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"github.com/jackc/pgx/v5/pgconn"
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
	return xnodeConfiguration(ctx, s.db, id)
}

func xnodeConfiguration(ctx context.Context, q xnodeSQLReader, id string) (xnode.Configuration, error) {
	config, err := scanXNodeConfiguration(q.QueryRowContext(ctx, `SELECT id,name,document_cipher,revision,created_at,updated_at FROM xnode_configs WHERE id=$1`, id))
	if err != nil {
		return config, err
	}
	rows, err := q.QueryContext(ctx, `SELECT snippet_id FROM xnode_config_snippets WHERE config_id=$1 ORDER BY position`, id)
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

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = saveXNodeConfigurationTx(ctx, tx, config); err != nil {
		return err
	}
	return tx.Commit()
}

func saveXNodeConfigurationTx(ctx context.Context, tx *sql.Tx, config xnode.Configuration) error {
	var err error
	if config.Revision == 0 {
		_, err = tx.ExecContext(ctx, `INSERT INTO xnode_configs(id,name,document_cipher,revision,created_at,updated_at) VALUES($1,$2,$3,1,$4,$5)`, config.ID, config.Name, config.DocumentCipher, stamp(time.Now().UTC()), stamp(time.Now().UTC()))
	} else {
		var result sql.Result
		result, err = tx.ExecContext(ctx, `UPDATE xnode_configs SET name=$1,document_cipher=$2,revision=revision+1,updated_at=$3 WHERE id=$4 AND revision=$5`, config.Name, config.DocumentCipher, stamp(time.Now().UTC()), config.ID, config.Revision)
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
	if _, err = tx.ExecContext(ctx, `DELETE FROM xnode_config_snippets WHERE config_id=$1`, config.ID); err != nil {
		return err
	}
	for position, id := range config.SnippetIDs {
		if _, err = tx.ExecContext(ctx, `INSERT INTO xnode_config_snippets(config_id,snippet_id,position) VALUES($1,$2,$3)`, config.ID, id, position); err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) DeleteXNodeConfiguration(ctx context.Context, id string) error {
	return deleteXNodeConfiguration(ctx, s.db, id)
}

func deleteXNodeConfiguration(ctx context.Context, q xnodeSQLExecutor, id string) error {
	result, err := q.ExecContext(ctx, `DELETE FROM xnode_configs WHERE id=$1`, id)
	if err != nil {
		var constraint *pgconn.PgError
		if errors.As(err, &constraint) && constraint.Code == "23503" {
			return xnode.ErrConflict
		}
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count == 0 {
		return xnode.ErrNotFound
	}
	return err
}
