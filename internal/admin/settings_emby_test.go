package admin

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

type embySettingsRepository struct {
	*adminSettingsRepository
	pair    database.EmbyConnectionRecord
	saveErr error
}

func (r *embySettingsRepository) EmbyConnection(context.Context) (database.EmbyConnectionRecord, error) {
	return r.pair, nil
}

func (r *embySettingsRepository) SaveEmbyConnection(_ context.Context, version string, next database.EmbyConnectionRecord, _ string) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	if r.pair.Version() != version {
		return database.ErrConflict
	}
	r.pair = next
	return nil
}

func TestEmbyConnectionReliesOnAtomicMutationAudit(t *testing.T) {
	ctx := context.Background()
	repository := &embySettingsRepository{adminSettingsRepository: newAdminSettingsRepository()}
	catalog := &adminCatalogRepository{auditErr: errors.New("separate audit unavailable")}
	service := NewService(catalog, NewSettingsService(repository, testVault(t)), nil, nil, nil)
	initial, err := service.EmbyConnection(ctx)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := service.SaveEmbyConnection(ctx, "admin", EmbyConnectionWrite{initial.Version, "https://emby.example", "first-token"})
	if err != nil || !saved.TokenConfigured || len(catalog.audits) != 0 {
		t.Fatal("committed pair depended on a second audit", err)
	}
	repository.saveErr = errors.New("atomic audit unavailable")
	before := repository.pair.Version()
	if _, err := service.SaveEmbyConnection(ctx, "admin", EmbyConnectionWrite{saved.Version, "", "replacement-token"}); !errors.Is(err, repository.saveErr) || repository.pair.Version() != before {
		t.Fatal("atomic audit failure changed connection", err)
	}
}

func TestEmbyConnectionCannotRetargetInheritedSecret(t *testing.T) {
	ctx := context.Background()
	repository := &embySettingsRepository{adminSettingsRepository: newAdminSettingsRepository()}
	service := NewSettingsService(repository, testVault(t))
	view, err := service.EmbyConnection(ctx)
	if err != nil || view.URLConfigured || view.TokenConfigured {
		t.Fatal(view, err)
	}
	view, err = service.SaveEmbyConnection(ctx, "admin", EmbyConnectionWrite{view.Version, "https://emby.example/emby", "first-token"})
	if err != nil || !view.URLConfigured || !view.TokenConfigured {
		t.Fatal(view, err)
	}
	first := repository.pair
	for _, input := range []EmbyConnectionWrite{
		{view.Version, "https://attacker.example", ""},
		{view.Version, "https://emby.example/other", ""},
		{view.Version, "https://attacker.example", "first-token"},
	} {
		if _, err := service.SaveEmbyConnection(ctx, "admin", input); !errors.Is(err, ErrEmbyFreshToken) {
			t.Fatal("inherited token retargeted", err)
		}
		if repository.pair.Version() != first.Version() {
			t.Fatal("rejected write changed connection")
		}
	}
	for _, key := range []string{database.EmbyURLSetting, database.EmbyTokenSetting} {
		if err := service.Put(ctx, "admin", key, "https://attacker.example"); err == nil {
			t.Fatal("generic settings bypass accepted", key)
		}
	}
	_, err = service.SaveEmbyConnection(ctx, "admin", EmbyConnectionWrite{view.Version, "http://unsafe.example", "fresh-token"})
	if !errors.Is(err, ErrEmbyConnection) || repository.pair.Version() != first.Version() {
		t.Fatal("invalid configuration changed pair", err)
	}
	updated, err := service.SaveEmbyConnection(ctx, "admin", EmbyConnectionWrite{view.Version, "", "rotated-token"})
	if err != nil || updated.Version == view.Version {
		t.Fatal(updated, err)
	}
	if _, err := service.SaveEmbyConnection(ctx, "admin", EmbyConnectionWrite{view.Version, "https://stale.example", "other-token"}); !errors.Is(err, database.ErrConflict) {
		t.Fatal("stale draft accepted", err)
	}
	baseURL, token, err := service.EmbyCredentials(ctx)
	if err != nil || baseURL != "https://emby.example/emby" || token != "rotated-token" || !repository.pair.BaseURL.Encrypted || !repository.pair.Token.Encrypted {
		t.Fatal("runtime pair mismatch", err)
	}
	_, err = service.SaveEmbyConnection(ctx, "admin", EmbyConnectionWrite{updated.Version, "https://new.example", "new-token"})
	if err != nil {
		t.Fatal(err)
	}
	baseURL, token, err = service.EmbyCredentials(ctx)
	if err != nil || baseURL != "https://new.example" || token != "new-token" {
		t.Fatal("replacement pair mismatch", err)
	}
}

func TestEmbyConnectionRejectsUnavailableAtomicStorage(t *testing.T) {
	service := NewSettingsService(newAdminSettingsRepository(), testVault(t))
	if _, _, err := service.EmbyCredentials(context.Background()); !errors.Is(err, ErrEmbyConnection) {
		t.Fatal("sequential credential fallback accepted", err)
	}
}
