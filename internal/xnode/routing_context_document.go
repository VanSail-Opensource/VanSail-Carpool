package xnode

import (
	"context"
	"encoding/json"
)

// Routing proofs and dependency digests must observe the same shared snippets
// as native compilation. IDs alone do not capture edits to a saved snippet.
func (s *Service) routingContextDocument(ctx context.Context, config Configuration) (map[string]any, json.RawMessage, error) {
	var native map[string]any
	if json.Unmarshal(config.Document, &native) != nil || native == nil {
		return nil, nil, ErrInvalid
	}
	for _, id := range config.SnippetIDs {
		snippet, err := s.repo.XNodeSnippet(ctx, id)
		if err != nil {
			return nil, nil, err
		}
		var objects []any
		if err := s.open("xnode.snippet."+id, snippet.DocumentCipher, &objects); err != nil {
			return nil, nil, err
		}
		if err := appendSharedSnippet(native, snippet.Kind, objects); err != nil {
			return nil, nil, err
		}
	}
	raw, err := json.Marshal(native)
	return native, raw, err
}
