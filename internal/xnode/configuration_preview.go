package xnode

import (
	"context"
	"encoding/json"
)

// PreviewConfiguration expands the same saved snippets used by compilation,
// without changing desired configuration or starting listeners.
func (s *Service) PreviewConfiguration(ctx context.Context, raw json.RawMessage, snippetIDs []string) (ValidationResult, error) {
	var native map[string]any
	if len(raw) > 1<<20 || json.Unmarshal(raw, &native) != nil || native == nil || len(snippetIDs) > 256 {
		return ValidationResult{ErrorCode: "XNODE_CONFIG_INVALID"}, nil
	}
	seen := map[string]bool{}
	for _, id := range snippetIDs {
		if seen[id] {
			return ValidationResult{ErrorCode: "XNODE_CONFIG_INVALID"}, nil
		}
		seen[id] = true
		snippet, err := s.repo.XNodeSnippet(ctx, id)
		if err != nil {
			return ValidationResult{}, err
		}
		var objects []any
		if err = s.open("xnode.snippet."+id, snippet.DocumentCipher, &objects); err != nil {
			return ValidationResult{}, err
		}
		if err = appendSharedSnippet(native, snippet.Kind, objects); err != nil {
			return ValidationResult{ErrorCode: "XNODE_CONFIG_INVALID"}, nil
		}
	}
	compiled, err := json.Marshal(native)
	if err != nil {
		return ValidationResult{}, err
	}
	return ValidateConfigurationDocument(compiled), nil
}
