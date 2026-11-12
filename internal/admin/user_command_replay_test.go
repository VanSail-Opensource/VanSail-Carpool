package admin

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestAdminCommandReplayPrecedesMutableCatalogValidation(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{
		providerops.KindAdminEntitlementEdit, providerops.KindAdminComboReplacement, providerops.KindAdminBulkExtension,
	} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			repository := &commandReplayRepository{}
			importer := &adminSquadImporter{squads: []UpstreamSquad{{UUID: manualGrantSquadID}}}
			workflows := NewUserWorkflows(repository, importer)
			first, err := replayCommand(ctx, workflows, kind, "admin-1", "user-1", "command-key", false)
			if err != nil || first == "" || repository.applied != 1 {
				t.Fatalf("first command = %q, %v; applied=%d", first, err, repository.applied)
			}
			reads := repository.comboReads
			repository.comboErr = database.ErrNotFound
			importer.err = errors.New("native inventory unavailable")
			replay, err := replayCommand(ctx, workflows, kind, "admin-1", "user-1", " command-key ", false)
			if err != nil || replay != first || repository.applied != 1 || repository.comboReads != reads {
				t.Fatalf("retained replay = %q, %v; repository=%+v", replay, err, repository)
			}
			if _, err := replayCommand(ctx, workflows, kind, "admin-1", "user-1", "command-key", true); !errors.Is(err, database.ErrConflict) {
				t.Fatalf("changed intent = %v", err)
			}
			if kind != providerops.KindAdminBulkExtension {
				if _, err := replayCommand(ctx, workflows, kind, "admin-1", "another-owner", "command-key", false); !errors.Is(err, database.ErrConflict) {
					t.Fatalf("changed owner = %v", err)
				}
			}
			if repository.comboReads != reads || repository.applied != 1 {
				t.Fatalf("conflicting replay consulted mutable state: %+v", repository)
			}
			if _, err := replayCommand(ctx, workflows, kind, "another-admin", "user-1", "command-key", false); !errors.Is(err, database.ErrNotFound) {
				t.Fatalf("another actor bypassed fresh validation: %v", err)
			}
			if _, err := replayCommand(ctx, workflows, kind, "admin-1", "user-1", "new-command-key", false); !errors.Is(err, database.ErrNotFound) {
				t.Fatalf("fresh intent bypassed live catalog: %v", err)
			}
			if repository.applied != 1 {
				t.Fatalf("invalid fresh operation applied: %+v", repository)
			}
			if kind == providerops.KindAdminEntitlementEdit && repository.editCalls != 2 {
				t.Fatalf("edit replay skipped protected store hydration: calls=%d", repository.editCalls)
			}
			if kind == providerops.KindAdminBulkExtension {
				if _, err := workflows.PreviewBulkExtension(ctx, BulkExtension{ComboIDs: []string{"combo-1"}, DurationMinutes: 30}); !errors.Is(err, database.ErrNotFound) {
					t.Fatalf("preview bypassed current catalog: %v", err)
				}
			}
		})
	}
}

func TestAdminCommandReplaySkipsUnavailableSquadProvider(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{providerops.KindAdminEntitlementEdit, providerops.KindAdminComboReplacement} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			repository := &commandReplayRepository{}
			importer := &adminSquadImporter{squads: []UpstreamSquad{{UUID: manualGrantSquadID}}}
			workflows := NewUserWorkflows(repository, importer)
			first, err := replayCommand(t.Context(), workflows, kind, "admin-1", "user-1", "command-key", false)
			if err != nil {
				t.Fatal(err)
			}
			unavailable := errors.New("provider unavailable")
			importer.err = unavailable
			replay, err := replayCommand(t.Context(), workflows, kind, "admin-1", "user-1", "command-key", false)
			if err != nil || replay != first {
				t.Fatalf("provider-blocked replay = %q, %v", replay, err)
			}
			if _, err := replayCommand(t.Context(), workflows, kind, "admin-1", "user-1", "fresh-key", false); !errors.Is(err, unavailable) {
				t.Fatalf("fresh provider validation = %v", err)
			}
			if repository.applied != 1 {
				t.Fatalf("unvalidated fresh intent applied: %+v", repository)
			}
		})
	}
}

func TestAdminCommandReplayLookupFailureStopsBeforeValidation(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{
		providerops.KindAdminEntitlementEdit, providerops.KindAdminComboReplacement, providerops.KindAdminBulkExtension,
	} {
		repository := &commandReplayRepository{lookupErr: errors.New("operation storage unavailable")}
		workflows := NewUserWorkflows(repository, nil)
		if _, err := replayCommand(t.Context(), workflows, kind, "admin-1", "user-1", "command-key", false); !errors.Is(err, repository.lookupErr) {
			t.Fatalf("%s lookup error = %v", kind, err)
		}
		if repository.comboReads != 0 || repository.applied != 0 {
			t.Fatalf("%s bypassed operation storage: %+v", kind, repository)
		}
	}
}

func TestFreshAdminSquadCommandsFailClosedWithoutImporter(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{providerops.KindAdminEntitlementEdit, providerops.KindAdminComboReplacement} {
		repository := &commandReplayRepository{}
		if _, err := replayCommand(t.Context(), NewUserWorkflows(repository, nil), kind, "admin-1", "user-1", "command-key", false); err == nil {
			t.Fatalf("%s accepted squads without an importer", kind)
		}
		if repository.applied != 0 {
			t.Fatalf("%s applied unavailable validation: %+v", kind, repository)
		}
	}
}
