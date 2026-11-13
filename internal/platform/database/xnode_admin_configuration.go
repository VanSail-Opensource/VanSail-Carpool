package database

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (s *Store) retryAttempt_SaveAdminXNodeConfiguration(ctx context.Context, actor string, item xnode.Configuration) (xnode.Configuration, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return xnode.Configuration{}, err
	}
	defer tx.Rollback()
	if err = xnodeAdminActorTx(ctx, tx, actor); err != nil {
		return xnode.Configuration{}, err
	}
	if err = saveXNodeConfigurationTx(ctx, tx, item); err != nil {
		return xnode.Configuration{}, err
	}
	item, err = xnodeConfiguration(ctx, tx, item.ID)
	if err != nil {
		return xnode.Configuration{}, err
	}
	if err = catalogAdminAuditTx(ctx, tx, actor, "xnode.configuration.save", "xnode_configuration", item.ID, map[string]any{"revision": item.Revision, "snippetCount": len(item.SnippetIDs)}, time.Now().UTC()); err != nil {
		return xnode.Configuration{}, err
	}
	if err = tx.Commit(); err != nil {
		return xnode.Configuration{}, err
	}
	return item, nil
}

func (s *Store) retryAttempt_DeleteAdminXNodeConfiguration(ctx context.Context, actor, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = xnodeAdminActorTx(ctx, tx, actor); err != nil {
		return err
	}
	if err = deleteXNodeConfiguration(ctx, tx, id); err != nil {
		return err
	}
	if err = catalogAdminAuditTx(ctx, tx, actor, "xnode.configuration.delete", "xnode_configuration", id, nil, time.Now().UTC()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) retryAttempt_SaveAdminXNodeSnippet(ctx context.Context, actor string, item xnode.Snippet) (xnode.Snippet, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return xnode.Snippet{}, err
	}
	defer tx.Rollback()
	if err = xnodeAdminActorTx(ctx, tx, actor); err != nil {
		return xnode.Snippet{}, err
	}
	if err = saveXNodeSnippet(ctx, tx, item); err != nil {
		return xnode.Snippet{}, err
	}
	item, err = scanXNodeSnippet(tx.QueryRowContext(ctx, `SELECT id,name,kind,document_cipher,revision,created_at,updated_at FROM xnode_snippets WHERE id=$1`, item.ID))
	if err != nil {
		return xnode.Snippet{}, err
	}
	if err = catalogAdminAuditTx(ctx, tx, actor, "xnode.snippet.save", "xnode_snippet", item.ID, map[string]any{"revision": item.Revision, "kind": item.Kind}, time.Now().UTC()); err != nil {
		return xnode.Snippet{}, err
	}
	if err = tx.Commit(); err != nil {
		return xnode.Snippet{}, err
	}
	return item, nil
}

func (s *Store) retryAttempt_DeleteAdminXNodeSnippet(ctx context.Context, actor, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = xnodeAdminActorTx(ctx, tx, actor); err != nil {
		return err
	}
	if err = deleteXNodeSnippet(ctx, tx, id); err != nil {
		return err
	}
	if err = catalogAdminAuditTx(ctx, tx, actor, "xnode.snippet.delete", "xnode_snippet", id, nil, time.Now().UTC()); err != nil {
		return err
	}
	return tx.Commit()
}
