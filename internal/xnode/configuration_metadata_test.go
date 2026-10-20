package xnode

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

type metadataRepository struct {
	Repository
	config  Configuration
	snippet Snippet
}

func (r *metadataRepository) XNodeConfiguration(context.Context, string) (Configuration, error) {
	return r.config, nil
}
func (r *metadataRepository) SaveXNodeConfiguration(_ context.Context, c Configuration) error {
	if c.Revision != r.config.Revision {
		return ErrConflict
	}
	c.Revision++
	r.config = c
	return nil
}
func (r *metadataRepository) XNodeSnippet(context.Context, string) (Snippet, error) {
	return r.snippet, nil
}
func (r *metadataRepository) SaveXNodeSnippet(_ context.Context, c Snippet) error {
	if c.Revision != r.snippet.Revision {
		return ErrConflict
	}
	c.Revision++
	r.snippet = c
	return nil
}

func TestMetadataEditsRetainCipherAndSkipNativeRevalidation(t *testing.T) {
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	raw := json.RawMessage(`{"inbounds":[],"outbounds":[{"tag":"direct","protocol":"freedom"}]}`)
	cipher, err := vault.Encrypt("xnode.config.config", string(raw))
	if err != nil {
		t.Fatal(err)
	}
	repo := &metadataRepository{config: Configuration{ID: "config", Name: "original", Revision: 1, DocumentCipher: cipher, SnippetIDs: []string{}}}
	service := NewService(repo, cacheTestSettings{}, vault, nil, nil)
	next := repo.config
	next.Document = raw
	next.Name = "renamed"
	saved, err := service.SaveConfiguration(context.Background(), next)
	if err != nil {
		t.Fatal(err)
	}
	if saved.DocumentCipher != cipher || saved.Name != "renamed" || saved.Revision != 2 {
		t.Fatal("metadata changed native document or revision", saved)
	}
	entry := service.compilation.entry("document:configuration:config")
	counts := entry.validator.Counts()
	next = saved
	next.Document = json.RawMessage(`{"outbounds":[{"protocol":"freedom","tag":"direct"}],"inbounds":[]}`)
	next.Name = "second"
	if _, err = service.SaveConfiguration(context.Background(), next); err != nil {
		t.Fatal(err)
	}
	if entry.validator.Counts() != counts {
		t.Fatal("metadata recompiled native configuration")
	}
	if _, err = service.SaveConfiguration(context.Background(), next); !errors.Is(err, ErrConflict) {
		t.Fatal("stale metadata write accepted", err)
	}
	if sameDocument([]byte(`{"value":9007199254740992}`), []byte(`{"value":9007199254740993}`)) {
		t.Fatal("large integer document difference lost")
	}
}

func TestAdmissionCommerceAndProbeSettingsDoNotInvalidateNativeGlobals(t *testing.T) {
	base := DefaultGlobal()
	next := base
	next.ConnectionLimit = 20
	next.FirewallUnbanPrice = "1"
	next.FirewallExemptionHours = 24
	next.SpeedtestDownloadURL = "https://example.com/down"
	next.SpeedtestUploadURL = "https://example.com/up"
	if globalRuntimeKey(base) != globalRuntimeKey(next) || globalPoliciesKey(base) != globalPoliciesKey(next) {
		t.Fatal("non-native global setting invalidated native policy")
	}
	next.BlockedDomains = []string{"full:example.com"}
	if globalRuntimeKey(base) == globalRuntimeKey(next) || globalPoliciesKey(base) == globalPoliciesKey(next) {
		t.Fatal("domain policy change skipped validation")
	}
}
