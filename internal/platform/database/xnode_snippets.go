package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func scanXNodeSnippet(row rowScanner) (xnode.Snippet, error) {
	var item xnode.Snippet
	var created, updated string
	err := row.Scan(&item.ID, &item.Name, &item.Kind, &item.DocumentCipher, &item.Revision, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return item, xnode.ErrNotFound
	}
	if err != nil {
		return item, err
	}
	item.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return item, err
	}
	item.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated)
	return item, err
}
func (s *Store) XNodeSnippets(ctx context.Context) ([]xnode.Snippet, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,kind,document_cipher,revision,created_at,updated_at FROM xnode_snippets ORDER BY kind,name,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []xnode.Snippet{}
	for rows.Next() {
		item, err := scanXNodeSnippet(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *Store) XNodeSnippet(ctx context.Context, id string) (xnode.Snippet, error) {
	return scanXNodeSnippet(s.db.QueryRowContext(ctx, `SELECT id,name,kind,document_cipher,revision,created_at,updated_at FROM xnode_snippets WHERE id=$1`, id))
}
func (s *Store) SaveXNodeSnippet(ctx context.Context, item xnode.Snippet) error {
	return saveXNodeSnippet(ctx, s.db, item)
}

func saveXNodeSnippet(ctx context.Context, q xnodeSQLExecutor, item xnode.Snippet) error {
	if item.Revision == 0 {
		_, err := q.ExecContext(ctx, `INSERT INTO xnode_snippets(id,name,kind,document_cipher,revision,created_at,updated_at) VALUES($1,$2,$3,$4,1,$5,$6)`, item.ID, item.Name, item.Kind, item.DocumentCipher, stamp(time.Now().UTC()), stamp(time.Now().UTC()))
		return err
	}
	result, err := q.ExecContext(ctx, `UPDATE xnode_snippets SET name=$1,kind=$2,document_cipher=$3,revision=revision+1,updated_at=$4 WHERE id=$5 AND revision=$6`, item.Name, item.Kind, item.DocumentCipher, stamp(time.Now().UTC()), item.ID, item.Revision)
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
	return nil
}
func (s *Store) DeleteXNodeSnippet(ctx context.Context, id string) error {
	return deleteXNodeSnippet(ctx, s.db, id)
}

func deleteXNodeSnippet(ctx context.Context, q xnodeSQLExecutor, id string) error {
	_, err := q.ExecContext(ctx, `DELETE FROM xnode_snippets WHERE id=$1`, id)
	return err
}
