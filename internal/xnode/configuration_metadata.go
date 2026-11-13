package xnode

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
)

func sameDocument(a, b []byte) bool {
	canonical := func(raw []byte) ([]byte, error) {
		var value any
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		return json.Marshal(value)
	}
	x, err := canonical(a)
	if err != nil {
		return false
	}
	y, err := canonical(b)
	return err == nil && bytes.Equal(x, y)
}

// Metadata edits retain encrypted native documents and ordered bindings. They
// still validate the current core contract and enforce optimistic revisions.
func (s *Service) saveConfigurationMetadata(ctx context.Context, next Configuration, actor string) (Configuration, bool, error) {
	if next.ID == "" {
		return next, false, nil
	}
	previous, err := s.repo.XNodeConfiguration(ctx, next.ID)
	if isMissing(err) && next.Revision == 0 {
		return next, false, nil
	}
	if err != nil {
		return next, true, err
	}
	if previous.Revision != next.Revision {
		return next, true, ErrConflict
	}
	raw, err := s.vault.Decrypt("xnode.config."+next.ID, previous.DocumentCipher)
	if err != nil {
		return next, true, err
	}
	if !sameDocument([]byte(raw), next.Document) || !slices.Equal(previous.SnippetIDs, next.SnippetIDs) {
		return next, false, nil
	}
	if !s.validateDocumentCached("configuration:"+next.ID, next.Document).Valid {
		return next, true, ErrInvalid
	}
	previous.Name = next.Name
	saved, err := s.persistConfiguration(ctx, actor, previous)
	return saved, true, err
}

func (s *Service) saveSnippetMetadata(ctx context.Context, next Snippet, actor string) (Snippet, bool, error) {
	if next.ID == "" {
		return next, false, nil
	}
	previous, err := s.repo.XNodeSnippet(ctx, next.ID)
	if isMissing(err) && next.Revision == 0 {
		return next, false, nil
	}
	if err != nil {
		return next, true, err
	}
	if previous.Revision != next.Revision {
		return next, true, ErrConflict
	}
	raw, err := s.vault.Decrypt("xnode.snippet."+next.ID, previous.DocumentCipher)
	if err != nil {
		return next, true, err
	}
	if previous.Kind != next.Kind || !sameDocument([]byte(raw), next.Document) {
		return next, false, nil
	}
	previous.Name = next.Name
	saved, err := s.persistSnippet(ctx, actor, previous)
	return saved, true, err
}
