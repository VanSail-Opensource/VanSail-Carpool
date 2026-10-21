package xnode

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

func TestBrowserAndEmptyResponseRulesShareSelectionPrecedence(t *testing.T) {
	config := DefaultSubscriptionTemplates()
	config.Templates = append(config.Templates, subscription.Template{ID: "deny", Name: "Empty response", Format: "empty-http"})
	config.UserAgentRules = []SubscriptionUserAgentRule{{Pattern: "Browser", Action: "browser"}, {Pattern: "Blocked", TemplateID: "deny"}}
	for _, test := range []struct {
		agent, explicit, kind string
		status                int
	}{
		{"Browser", "", "browser", 200}, {"Blocked", "", "empty-http", 403}, {"Blocked", "native", "document", 200}, {"Browser", "xray", "document", 200}, {"unknown", "", "document", 200},
	} {
		selection, err := SelectSubscription(config, test.explicit, test.agent)
		preview, validation := PreviewSubscriptionSelection(config, test.explicit, test.agent)
		if err != nil || !validation.Valid || selection != preview || selection.ResponseKind != test.kind || selection.StatusCode != test.status {
			t.Fatal(test, selection, preview, validation, err)
		}
	}
	config.Defaults["empty-http"] = "deny"
	if validateTemplateReferences(config) == nil {
		t.Fatal("response template became a format default")
	}
	delete(config.Defaults, "empty-http")
	config.UserAgentRules[0].TemplateID = "deny"
	if validateTemplateReferences(config) == nil {
		t.Fatal("browser action retained a template reference")
	}
}

func TestEmptyResponseStatusAndDocumentValidation(t *testing.T) {
	for _, status := range []int{0, 200, 204, 304, 403, 599, 199, 600} {
		item := subscription.Template{ID: "empty", Name: "Empty", Format: "empty-http", StatusCode: status}
		raw, validation, err := (&Service{}).PreviewSubscriptionTemplate(item)
		if err != nil || validation.Valid != (status == 0 || status >= 200 && status <= 599) || len(raw) != 0 {
			t.Fatal(status, string(raw), validation, err)
		}
	}
	item := subscription.Template{Format: "empty-http", Document: json.RawMessage(`{"secret":"never returned"}`)}
	if _, err := subscription.PreviewTemplate(item); err == nil {
		t.Fatal("empty response accepted document content")
	}
}

type deniedResponseRepository struct{ Repository }

func (deniedResponseRepository) XNodeIdentityBySubscription(context.Context, []byte) (Identity, error) {
	return Identity{}, ErrNotFound
}

func TestResponseActionsCannotBypassSubscriptionAccess(t *testing.T) {
	// No settings or vault are supplied: selecting a response before validating
	// the bearer would access them instead of rejecting the revoked identity.
	s := &Service{repo: deniedResponseRepository{}}
	_, _, err := s.BearerSubscriptionForAgent(context.Background(), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "", "Browser")
	if !errors.Is(err, ErrNotFound) {
		t.Fatal("access validation was bypassed", err)
	}
}
