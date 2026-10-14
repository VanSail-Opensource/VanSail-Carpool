package xnode

import (
	"context"
	"encoding/json"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/VanSail-Opensource/X-Node/pkg/xrayconfig"
)

// ValidateCompiled validates expanded users, shared snippets and member rules
// using the agent's exact core, without starting a listener or reading node files.
func ValidateCompiled(config wire.Config) ValidationResult {
	invalid := ValidationResult{ErrorCode: "XNODE_CONFIG_INVALID"}
	if config.Validate() != nil {
		return invalid
	}
	var native map[string]any
	if json.Unmarshal(config.Xray, &native) != nil {
		return invalid
	}
	needsAgent := nodeLocalAssets(native) || len(config.InboundCertificates) > 0
	if len(config.InboundCertificates) > 0 {
		inbounds, _ := native["inbounds"].([]any)
		for _, item := range inbounds {
			inbound, _ := item.(map[string]any)
			tag, _ := inbound["tag"].(string)
			if config.InboundCertificates[tag] == "" {
				continue
			}
			stream, _ := inbound["streamSettings"].(map[string]any)
			if stream == nil {
				stream = map[string]any{}
				inbound["streamSettings"] = stream
			}
			if stream["security"] == "reality" {
				return invalid
			}
			stream["security"] = "tls"
			settings, _ := stream["tlsSettings"].(map[string]any)
			if settings == nil {
				settings = map[string]any{}
				stream["tlsSettings"] = settings
			}
			settings["certificates"] = []any{map[string]any{"certificateFile": "node-certificate", "keyFile": "node-key"}}
		}
	}
	if needsAgent {
		var err error
		native, err = validationAssets(native)
		if err != nil {
			return invalid
		}
	}
	data, err := json.Marshal(native)
	if err != nil {
		return invalid
	}
	copy := wire.Clone(config)
	copy.Xray = data
	copy.InboundCertificates = nil
	if xrayconfig.Validate(copy) != nil {
		return invalid
	}
	return ValidationResult{Valid: true, NeedsAgentValidation: needsAgent}
}
func (s *Service) validateAffected(ctx context.Context, config *Configuration, snippet *Snippet, fleet *GlobalConfig) error {
	nodes, err := s.repo.XNodeNodes(ctx)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if config != nil && node.ConfigID != config.ID {
			continue
		}
		if snippet != nil {
			base, err := s.repo.XNodeConfiguration(ctx, node.ConfigID)
			if err != nil {
				return err
			}
			used := false
			for _, id := range base.SnippetIDs {
				if id == snippet.ID {
					used = true
				}
			}
			if !used {
				continue
			}
		}
		if _, err = s.compile(ctx, node.ID, config, snippet, fleet, nil); err != nil {
			return err
		}
		if err = s.checkTagOwnership(ctx, node.ID, config, snippet, nil); err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) validateSharedBase(ctx context.Context, config Configuration, replacement *Snippet) error {
	var native map[string]any
	if json.Unmarshal(config.Document, &native) != nil {
		return ErrInvalid
	}
	for _, id := range config.SnippetIDs {
		snippet, err := s.repo.XNodeSnippet(ctx, id)
		if err != nil {
			return err
		}
		if replacement != nil && replacement.ID == id {
			snippet = *replacement
		}
		raw, err := s.vault.Decrypt("xnode.snippet."+id, snippet.DocumentCipher)
		if err != nil {
			return err
		}
		var objects []any
		if json.Unmarshal([]byte(raw), &objects) != nil {
			return ErrInvalid
		}
		if err = appendSharedSnippet(native, snippet.Kind, objects); err != nil {
			return err
		}
	}
	raw, err := json.Marshal(native)
	if err != nil {
		return err
	}
	if !ValidateConfigurationDocument(raw).Valid {
		return ErrInvalid
	}
	return nil
}
