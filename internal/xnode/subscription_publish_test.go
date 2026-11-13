package xnode_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

type subscriptionPublicationRepository struct {
	*database.Store
	onEndpoints func()
}

func (r *subscriptionPublicationRepository) XNodeEndpoints(ctx context.Context) ([]xnode.Endpoint, error) {
	rows, err := r.Store.XNodeEndpoints(ctx)
	if err == nil && r.onEndpoints != nil {
		hook := r.onEndpoints
		r.onEndpoints = nil
		hook()
	}
	return rows, err
}

type subscriptionPublicationSettings struct {
	fixtureSettings
	reads    int
	onSelect func()
}

func (s *subscriptionPublicationSettings) Optional(ctx context.Context, key string) (string, error) {
	raw, err := s.fixtureSettings.Optional(ctx, key)
	if err == nil && key == xnode.SubscriptionTemplatesKey {
		s.reads++
		// The first read projects metadata; the second selects the live response.
		if s.reads == 2 && s.onSelect != nil {
			s.onSelect()
		}
	}
	return raw, err
}

func publicationService(t *testing.T, repo xnode.Repository, settings xnode.Settings) *xnode.Service {
	t.Helper()
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	public, _ := url.Parse("https://panel.example")
	return xnode.NewService(repo, settings, vault, public, nil)
}

func TestOwnedSubscriptionDiscardsURLAfterConcurrentAuthorityChange(t *testing.T) {
	for _, mutation := range []string{"token", "restriction"} {
		t.Run(mutation, func(t *testing.T) {
			f := controlFixture(t, true)
			ctx := context.Background()
			repo := &subscriptionPublicationRepository{Store: f.store}
			service := publicationService(t, repo, &fixtureSettings{})
			before, err := service.OwnedSubscription(ctx, f.user.ID)
			if err != nil || before.URL == "" {
				t.Fatal("fixture has no subscription URL", before, err)
			}
			called := false
			repo.onEndpoints = func() {
				called = true
				if mutation == "token" {
					_, err = f.service.RevokeSubscription(ctx, stringID(f.identity))
				} else {
					err = f.store.SetXNodeAccessDisabled(ctx, f.identity.ProxyUserID, true)
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			after, err := service.OwnedSubscription(ctx, f.user.ID)
			if !called || !errors.Is(err, xnode.ErrConflict) || !reflect.DeepEqual(after, xnode.SubscriptionMetadata{}) {
				t.Fatal("changed owner retained a partial subscription", called, after, err)
			}
		})
	}
}

func TestBearerPublicationChecksAfterDocumentAndResponseSelection(t *testing.T) {
	f := controlFixture(t, true)
	ctx := context.Background()
	if _, err := f.service.SaveEndpoint(ctx, xnode.Endpoint{NodeID: f.node.ID, InboundTag: "paid", Name: "Host", Address: "proxy.example", Port: 443, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	config := xnode.DefaultSubscriptionTemplates()
	config.Templates = append(config.Templates, subscription.Template{ID: "deny", Name: "Empty", Format: "empty-http"})
	config.UserAgentRules = []xnode.SubscriptionUserAgentRule{{Pattern: "Browser", Action: "browser"}, {Pattern: "Blocked", TemplateID: "deny"}}
	rawConfig, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	for _, agent := range []string{"Document", "Browser", "Blocked"} {
		t.Run(agent, func(t *testing.T) {
			metadata, err := f.service.OwnedSubscription(ctx, f.user.ID)
			if err != nil || !metadata.Active {
				t.Fatal("fixture has no active subscription", metadata, err)
			}
			token := metadata.DownloadURL[strings.LastIndex(metadata.DownloadURL, "/")+1:]
			settings := &subscriptionPublicationSettings{fixtureSettings: fixtureSettings{templates: string(rawConfig)}}
			called := false
			settings.onSelect = func() {
				called = true
				if _, err := f.service.RevokeSubscription(ctx, stringID(f.identity)); err != nil {
					t.Fatal(err)
				}
			}
			service := publicationService(t, f.store, settings)
			body, result, err := service.BearerSubscriptionForAgent(ctx, token, "", agent)
			if !called || !errors.Is(err, xnode.ErrConflict) || len(body) != 0 || !reflect.DeepEqual(result, xnode.SubscriptionMetadata{}) {
				t.Fatal("selected response escaped after token rotation", called, len(body), result, err)
			}
		})
	}
}
