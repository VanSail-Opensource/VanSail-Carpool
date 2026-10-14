package accounts

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/turnstile"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type firstEntryStore struct {
	qualified, registered, challenged bool
	writes                            int
	err                               error
}

func (s *firstEntryStore) RegisterPanelEntry(_ context.Context, _ int64, challenge bool) error {
	if s.err != nil {
		return s.err
	}
	s.registered = true
	s.challenged = challenge
	if !challenge {
		s.qualified = true
	}
	return nil
}
func (s *firstEntryStore) PanelEntryQualified(context.Context, int64) (bool, error) {
	return s.qualified, s.err
}
func (s *firstEntryStore) VerifyPanelEntry(context.Context, int64) error {
	if s.err != nil {
		return s.err
	}
	if s.registered {
		s.qualified = true
		s.writes++
	}
	return nil
}

type firstEntrySettings struct {
	enabled, siteKey, secret string
	err                      error
}

func (s firstEntrySettings) Optional(_ context.Context, key string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	switch key {
	case "captcha.turnstile.enabled":
		return s.enabled, nil
	case "captcha.turnstile.site_key":
		return s.siteKey, nil
	default:
		return s.secret, nil
	}
}

type firstEntryVerifier struct {
	calls                   int
	secret, token, hostname string
	err                     error
}

func (v *firstEntryVerifier) Verify(_ context.Context, secret, token, hostname string) error {
	v.calls++
	v.secret, v.token, v.hostname = secret, token, hostname
	return v.err
}

func TestFirstEntryEnabledBootstrapVerifyAndReplay(t *testing.T) {
	t.Parallel()
	repository := &firstEntryStore{}
	verifier := &firstEntryVerifier{}
	entry := FirstEntry{Repository: repository, Settings: firstEntrySettings{enabled: "true", siteKey: "public-site", secret: "private-secret"}, Verifier: verifier, Hostname: "panel.example"}
	ctx, user := context.Background(), model.User{TelegramID: 42, Role: "user"}
	state, err := entry.State(ctx, user, false)
	if err != nil || !state.Required || repository.registered {
		t.Fatalf("non-entry state=%+v,%v", state, err)
	}
	state, err = entry.State(ctx, user, true)
	if err != nil || !state.Required || state.SiteKey != "public-site" || state.Action != turnstile.Action || !repository.challenged {
		t.Fatalf("first entry=%+v,%v", state, err)
	}
	if err := entry.Verify(ctx, user, "single-use-token"); err != nil {
		t.Fatal(err)
	}
	if verifier.calls != 1 || verifier.secret != "private-secret" || verifier.token != "single-use-token" || verifier.hostname != "panel.example" || repository.writes != 1 {
		t.Fatal("verification identity/configuration mismatch")
	}
	if err := entry.Verify(ctx, user, "same-request-retry"); err != nil || verifier.calls != 1 || repository.writes != 1 {
		t.Fatalf("replay=%v calls=%d writes=%d", err, verifier.calls, repository.writes)
	}
	state, err = entry.State(ctx, user, false)
	if err != nil || state.Required || state.SiteKey != "" {
		t.Fatalf("verified state=%+v,%v", state, err)
	}
}

func TestFirstEntryDisabledVisitIsRequiredForPMQualification(t *testing.T) {
	t.Parallel()
	for _, enabled := range []string{"", "false"} {
		repository := &firstEntryStore{}
		entry := FirstEntry{Repository: repository, Settings: firstEntrySettings{enabled: enabled}}
		user := model.User{TelegramID: 42, Role: "user"}
		if state, err := entry.State(context.Background(), user, false); err != nil || state.Required || repository.qualified {
			t.Fatalf("before visit=%+v,%v", state, err)
		}
		if state, err := entry.State(context.Background(), user, true); err != nil || state.Required || !repository.qualified || repository.challenged {
			t.Fatalf("disabled visit=%+v,%v", state, err)
		}
		if err := entry.Verify(context.Background(), user, "unused"); err != nil || repository.writes != 0 {
			t.Fatalf("disabled verify=%v", err)
		}
	}
}

func TestFirstEntryUnavailableProviderAndConfigurationNeverQualify(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name          string
		settings      firstEntrySettings
		hostname      string
		nilVerifier   bool
		providerError error
	}{
		{"settings failure", firstEntrySettings{err: errors.New("settings unavailable")}, "panel.example", false, nil},
		{"invalid toggle", firstEntrySettings{enabled: "invalid"}, "panel.example", false, nil},
		{"missing site key", firstEntrySettings{enabled: "true"}, "panel.example", false, nil},
		{"missing hostname", firstEntrySettings{enabled: "true", siteKey: "public"}, "", false, nil},
		{"missing verifier", firstEntrySettings{enabled: "true", siteKey: "public", secret: "private"}, "panel.example", true, nil},
		{"provider rejection", firstEntrySettings{enabled: "true", siteKey: "public", secret: "private"}, "panel.example", false, turnstile.ErrRejected},
		{"provider unavailable", firstEntrySettings{enabled: "true", siteKey: "public", secret: "private"}, "panel.example", false, turnstile.ErrUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &firstEntryStore{}
			entry := FirstEntry{Repository: repository, Settings: test.settings, Hostname: test.hostname}
			if !test.nilVerifier {
				entry.Verifier = &firstEntryVerifier{err: test.providerError}
			}
			if err := entry.Verify(context.Background(), model.User{TelegramID: 42, Role: "user"}, "expired-token"); err == nil || repository.qualified || repository.writes != 0 {
				t.Fatalf("failure qualified=%v writes=%d err=%v", repository.qualified, repository.writes, err)
			}
		})
	}
}

func TestFirstEntryAdminExemptionRequiresConfiguredIdentity(t *testing.T) {
	t.Parallel()
	entry := FirstEntry{Repository: &firstEntryStore{}, Settings: firstEntrySettings{enabled: "true", siteKey: "public"}, Hostname: "panel.example", AdminTelegramIDs: []int64{42}}
	for _, test := range []struct {
		id       int64
		role     string
		required bool
	}{{42, "admin", false}, {43, "admin", true}, {42, "user", true}} {
		state, err := entry.State(context.Background(), model.User{TelegramID: test.id, Role: test.role}, false)
		if err != nil || state.Required != test.required {
			t.Fatalf("id=%d role=%s state=%+v,%v", test.id, test.role, state, err)
		}
	}
	if state, err := entry.State(context.Background(), model.User{TelegramID: 42, Role: "admin"}, true); err != nil || state.Required {
		t.Fatalf("admin visit=%+v,%v", state, err)
	}
	qualified, err := entry.Repository.PanelEntryQualified(context.Background(), 42)
	if err != nil || !qualified {
		t.Fatal("panel admin visit did not qualify PM")
	}
}
