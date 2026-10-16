package xnode

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/validation"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type ValidationResult struct {
	Valid                bool   `json:"valid"`
	NeedsAgentValidation bool   `json:"needsAgentValidation"`
	ErrorCode            string `json:"errorCode"`
	Detail               string `json:"detail,omitempty"`
}

func (s *Service) SaveConfiguration(ctx context.Context, config Configuration) (Configuration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(config.Name) == "" || len(config.Name) > 128 || len(config.Document) > 1<<20 || validation.JSONDocument(config.Document) != nil {
		return config, ErrInvalid
	}
	result := ValidateConfigurationDocument(config.Document)
	if !result.Valid {
		return config, ErrInvalid
	}
	if config.ID == "" {
		var err error
		config.ID, err = ids.New()
		if err != nil {
			return config, err
		}
		config.Revision = 0
	}
	for _, id := range config.SnippetIDs {
		if _, err := s.repo.XNodeSnippet(ctx, id); err != nil {
			return config, err
		}
	}
	var err error
	config.DocumentCipher, err = s.vault.Encrypt("xnode.config."+config.ID, string(config.Document))
	if err != nil {
		return config, err
	}
	if err = s.validateSharedBase(ctx, config, nil); err != nil {
		return config, err
	}
	if err = s.validateAffected(ctx, &config, nil, nil); err != nil {
		return config, err
	}
	if err = s.repo.SaveXNodeConfiguration(ctx, config); err != nil {
		return config, err
	}
	return s.repo.XNodeConfiguration(ctx, config.ID)
}
func (s *Service) SaveSnippet(ctx context.Context, item Snippet) (Snippet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !wire.ValidName(item.Name) || (item.Kind != "outbound" && item.Kind != "routing") || len(item.Document) > 256<<10 || validation.JSONDocument(item.Document) != nil {
		return item, ErrInvalid
	}
	var objects []map[string]any
	if json.Unmarshal(item.Document, &objects) != nil || len(objects) > 256 {
		return item, ErrInvalid
	}
	if item.ID == "" {
		var err error
		item.ID, err = ids.New()
		if err != nil {
			return item, err
		}
		item.Revision = 0
	}
	var err error
	item.DocumentCipher, err = s.vault.Encrypt("xnode.snippet."+item.ID, string(item.Document))
	if err != nil {
		return item, err
	}
	configs, err := s.Configurations(ctx)
	if err != nil {
		return item, err
	}
	for _, config := range configs {
		for _, id := range config.SnippetIDs {
			if id == item.ID {
				if err = s.validateSharedBase(ctx, config, &item); err != nil {
					return item, err
				}
			}
		}
	}
	if err = s.validateAffected(ctx, nil, &item, nil); err != nil {
		return item, err
	}
	if err = s.repo.SaveXNodeSnippet(ctx, item); err != nil {
		return item, err
	}
	return s.repo.XNodeSnippet(ctx, item.ID)
}

func (s *Service) DeleteConfiguration(ctx context.Context, id string) error {
	return s.repo.DeleteXNodeConfiguration(ctx, id)
}
func (s *Service) DeleteSnippet(ctx context.Context, id string) error {
	return s.repo.DeleteXNodeSnippet(ctx, id)
}
func (s *Service) ResolveConfiguration(ctx context.Context, id string) (Configuration, error) {
	config, err := s.repo.XNodeConfiguration(ctx, id)
	if err != nil {
		return config, err
	}
	raw, err := s.vault.Decrypt("xnode.config."+id, config.DocumentCipher)
	if err != nil {
		return config, err
	}
	config.Document = json.RawMessage(raw)
	return config, nil
}
func isMissing(err error) bool { return errors.Is(err, ErrNotFound) }
