package xnode

import (
	"context"
	"encoding/json"
)

// Hyphens are allowed in both usernames and member tags. Concatenation alone is
// therefore insufficient: reserve every saved tag, including inactive members,
// against other owners and administrator outbounds before changing state.
func (s *Service) checkTagOwnership(ctx context.Context, nodeID string, config *Configuration, shared *Snippet, replacement *Customization) error {
	node, err := s.repo.XNodeNode(ctx, nodeID)
	if err != nil {
		return err
	}
	base, err := s.repo.XNodeConfiguration(ctx, node.ConfigID)
	if err != nil {
		return err
	}
	if config != nil && config.ID == base.ID {
		base = *config
	}
	raw, err := s.vault.Decrypt("xnode.config."+base.ID, base.DocumentCipher)
	if err != nil {
		return err
	}
	var native map[string]any
	if json.Unmarshal([]byte(raw), &native) != nil {
		return ErrInvalid
	}
	for _, id := range base.SnippetIDs {
		snippet, err := s.repo.XNodeSnippet(ctx, id)
		if err != nil {
			return err
		}
		if shared != nil && shared.ID == id {
			snippet = *shared
		}
		data, err := s.vault.Decrypt("xnode.snippet."+id, snippet.DocumentCipher)
		if err != nil {
			return err
		}
		var objects []any
		if json.Unmarshal([]byte(data), &objects) != nil {
			return ErrInvalid
		}
		if err = appendSharedSnippet(native, snippet.Kind, objects); err != nil {
			return err
		}
	}
	reserved := map[string]bool{}
	objects, _ := native["outbounds"].([]any)
	hasMarker := false
	for _, raw := range objects {
		object, _ := raw.(map[string]any)
		if _, ok := object["UserOutbound"]; ok {
			hasMarker = true
			continue
		}
		tag, _ := object["tag"].(string)
		if tag != "" {
			reserved[tag] = true
		}
	}
	items, err := s.repo.XNodeCustomizations(ctx, nodeID)
	if err != nil {
		return err
	}
	if replacement != nil {
		found := false
		for i, item := range items {
			if customizationOwner(item) == customizationOwner(*replacement) {
				items[i] = *replacement
				found = true
				break
			}
		}
		if !found {
			items = append(items, *replacement)
		}
	}
	// Managed plans supersede archived raw customizations for this owner.
	managed, err := s.managedReservedTags(ctx, nodeID)
	if err != nil {
		return err
	}
	for _, tags := range managed {
		if len(tags) > 0 && !hasMarker {
			return ErrConflict
		}
		for _, tag := range tags {
			if reserved[tag] {
				return ErrConflict
			}
			reserved[tag] = true
		}
	}
	for _, item := range items {
		if _, owned := managed[item.ProxyUserID]; owned {
			continue
		}
		var outs []map[string]any
		if err = s.open("xnode.outbounds."+customizationOwner(item)+"."+nodeID, item.OutboundsCipher, &outs); err != nil {
			return err
		}
		if len(outs) > 0 && !hasMarker {
			return ErrConflict
		}
		for _, out := range outs {
			tag, _ := out["tag"].(string)
			if tag == "" || reserved[tag] {
				return ErrConflict
			}
			reserved[tag] = true
		}
	}
	return nil
}
