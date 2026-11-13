package xnode

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

type routingContextDocumentRepository struct {
	Repository
	config   Configuration
	snippets map[string]Snippet
	failure  error
}

func (r *routingContextDocumentRepository) XNodeConfiguration(context.Context, string) (Configuration, error) {
	return r.config, r.failure
}

func (r *routingContextDocumentRepository) XNodeSnippet(_ context.Context, id string) (Snippet, error) {
	return r.snippets[id], r.failure
}

func routingContextFixture(t *testing.T) (*Service, *routingContextDocumentRepository, Configuration) {
	t.Helper()
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	r := &routingContextDocumentRepository{snippets: map[string]Snippet{}}
	s := NewService(r, nil, vault, nil, nil)
	config := Configuration{ID: "base", Document: json.RawMessage(`{"inbounds":[{"tag":"paid"}],"outbounds":[{"tag":"direct","protocol":"freedom"},{"UserOutbound":{}}],"routing":{"rules":[{"UserRouting":{}}]}}`), SnippetIDs: []string{"exit", "fallback"}}
	config.DocumentCipher, err = vault.Encrypt("xnode.config.base", string(config.Document))
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []Snippet{
		{ID: "exit", Kind: "outbound", Document: json.RawMessage(`[{"tag":"shared-exit","protocol":"freedom"}]`)},
		{ID: "fallback", Kind: "routing", Document: json.RawMessage(`[{"outboundTag":"shared-exit","network":"tcp,udp"}]`)},
	} {
		item.DocumentCipher, err = vault.Encrypt("xnode.snippet."+item.ID, string(item.Document))
		if err != nil {
			t.Fatal(err)
		}
		r.snippets[item.ID] = item
	}
	r.config = config
	return s, r, config
}

func TestRoutingContextIncludesSharedFallbackAndItsContentDependency(t *testing.T) {
	s, r, config := routingContextFixture(t)
	ctx := context.Background()
	base := string(config.Document)
	root, original, err := s.routingContextDocument(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = routingServerContext(root, []string{"paid"}, false, false); err != nil {
		t.Fatal("safe shared fallback rejected", err)
	}
	changed := r.snippets["exit"]
	changed.DocumentCipher, err = s.vault.Encrypt("xnode.snippet.exit", `[{"tag":"shared-exit","protocol":"socks","settings":{"servers":[{"address":"1.1.1.1","port":1080}]}}]`)
	if err != nil {
		t.Fatal(err)
	}
	r.snippets["exit"] = changed // Same saved ID, different operational content.
	root, updated, err := s.routingContextDocument(ctx, config)
	if err != nil || string(original) == string(updated) || string(config.Document) != base {
		t.Fatal("snippet edit was absent from dependency input or changed the source", err)
	}
	if _, err = routingServerContext(root, []string{"paid"}, false, false); !errors.Is(err, ErrInvalid) {
		t.Fatal("untracked shared proxy escaped cycle context proof", err)
	}
	r.failure = errors.New("storage unavailable")
	if _, _, err = s.routingContextDocument(ctx, config); !errors.Is(err, r.failure) {
		t.Fatal("snippet read failure became an incomplete successful proof", err)
	}
}

func TestCompileCandidatesReachNestedDocumentReadsAndRemainScoped(t *testing.T) {
	s, r, config := routingContextFixture(t)
	ctx := context.Background()
	for _, preloaded := range []bool{false, true} {
		t.Run(map[bool]string{false: "empty cache", true: "loaded cache"}[preloaded], func(t *testing.T) {
			scoped := s.withCompileReads()
			if preloaded {
				if _, err := scoped.ResolveConfiguration(ctx, config.ID); err != nil {
					t.Fatal(err)
				}
				if _, _, err := scoped.routingContextDocument(ctx, config); err != nil {
					t.Fatal(err)
				}
			}
			candidate := config
			var err error
			candidate.DocumentCipher, err = s.vault.Encrypt("xnode.config.base", `{"inbounds":[{"tag":"candidate"}],"outbounds":[],"routing":{"rules":[]}}`)
			if err != nil {
				t.Fatal(err)
			}
			snippet := r.snippets["exit"]
			snippet.DocumentCipher, err = s.vault.Encrypt("xnode.snippet.exit", `[{"tag":"candidate-exit","protocol":"blackhole"}]`)
			if err != nil {
				t.Fatal(err)
			}
			restore := scoped.repo.(*compileReads).useDocuments(&candidate, &snippet)
			resolved, err := scoped.ResolveConfiguration(ctx, config.ID)
			if err != nil || string(resolved.Document) == string(config.Document) {
				t.Fatal("nested config read missed candidate", err)
			}
			root, _, err := scoped.routingContextDocument(ctx, resolved)
			if err != nil || root["outbounds"].([]any)[0].(map[string]any)["tag"] != "candidate-exit" {
				t.Fatal("nested snippet read missed candidate", err)
			}
			restore()
			resolved, err = scoped.ResolveConfiguration(ctx, config.ID)
			if err != nil || string(resolved.Document) != string(config.Document) {
				t.Fatal("candidate escaped request scope", err)
			}
			actual, err := scoped.repo.XNodeSnippet(ctx, snippet.ID)
			if err != nil || actual.DocumentCipher != r.snippets[snippet.ID].DocumentCipher {
				t.Fatal("candidate snippet escaped request scope", err)
			}
		})
	}
}
