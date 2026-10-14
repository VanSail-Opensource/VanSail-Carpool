package admin

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/squadprofile"
	"strings"
	"testing"
)

func TestAdminSettingsAreForwardedAndAudited(t *testing.T) {
	t.Parallel()

	settingsRepository := newAdminSettingsRepository()
	settings := NewSettingsService(settingsRepository, testVault(t))
	repository := &adminCatalogRepository{}
	service := newAdminServiceForTest(repository, settings, &adminSquadImporter{}, &adminBackupRunner{}, &adminRefunder{})

	listed, err := service.Settings(context.Background())
	if err != nil || len(listed) != len(settingDefinitions) {
		t.Fatalf("Settings() = (%d settings, %v)", len(listed), err)
	}
	if err := service.PutSetting(context.Background(), "admin-1", "telegram.group_chat_id", "-1001"); err != nil {
		t.Fatalf("PutSetting(): %v", err)
	}
	if len(repository.audits) != 1 || repository.audits[0].action != "setting.update" || !strings.Contains(repository.audits[0].detail, `"secret":false`) {
		t.Fatalf("setting audit = %+v", repository.audits)
	}

	if err := service.PutSetting(context.Background(), "admin-1", "unknown", "value"); err == nil {
		t.Fatal("PutSetting() accepted unknown key")
	}
	repository.auditErr = errors.New("audit failure")
	if err := service.PutSetting(context.Background(), "admin-1", "telegram.channel_chat_id", "-1002"); err == nil {
		t.Fatal("PutSetting() ignored audit failure")
	}
}

func TestAdminSaveComboValidationAndAudit(t *testing.T) {
	t.Parallel()

	valid := database.ComboInput{Name: " Monthly ", PriceTXBMinor: 1000, ValidityDays: 30, TrafficLimitBytes: 1024, ResetStrategy: "MONTH_ROLLING", Active: true}
	invalid := []struct {
		name   string
		mutate func(*database.ComboInput)
	}{
		{name: "empty name", mutate: func(input *database.ComboInput) { input.Name = " " }},
		{name: "negative price", mutate: func(input *database.ComboInput) { input.PriceTXBMinor = -1 }},
		{name: "zero validity", mutate: func(input *database.ComboInput) { input.ValidityDays = 0 }},
		{name: "excess validity", mutate: func(input *database.ComboInput) { input.ValidityDays = 3651 }},
		{name: "zero traffic", mutate: func(input *database.ComboInput) { input.TrafficLimitBytes = 0 }},
		{name: "reset strategy", mutate: func(input *database.ComboInput) { input.ResetStrategy = "YEAR" }},
		{name: "legacy calendar month", mutate: func(input *database.ComboInput) { input.ResetStrategy = "MONTH" }},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			input := valid
			test.mutate(&input)
			if _, err := newAdminServiceForTest(&adminCatalogRepository{}, nil, &adminSquadImporter{}, &adminBackupRunner{}, &adminRefunder{}).SaveCombo(context.Background(), "admin", input); err == nil {
				t.Fatal("SaveCombo() unexpectedly succeeded")
			}
		})
	}

	repository := &adminCatalogRepository{savedCombo: model.Combo{ID: "combo-1", Name: "Monthly"}}
	service := newAdminServiceForTest(repository, nil, &adminSquadImporter{}, &adminBackupRunner{}, &adminRefunder{})
	combo, err := service.SaveCombo(context.Background(), "admin", valid)
	if err != nil || combo.ID != "combo-1" || repository.comboInput.Name != "Monthly" || repository.audits[0].action != "combo.save" {
		t.Fatalf("SaveCombo() = (%+v, %v), input %+v, audits %+v", combo, err, repository.comboInput, repository.audits)
	}

	repository.saveComboErr = errors.New("save failure")
	if _, err := service.SaveCombo(context.Background(), "admin", valid); err == nil {
		t.Fatal("SaveCombo() ignored repository failure")
	}
	repository.saveComboErr = nil
	repository.auditErr = errors.New("audit failure")
	if _, err := service.SaveCombo(context.Background(), "admin", valid); err == nil {
		t.Fatal("SaveCombo() ignored audit failure")
	}
}

func TestAdminDeleteComboAndSaveSquad(t *testing.T) {
	t.Parallel()

	repository := &adminCatalogRepository{savedSquad: model.SquadProduct{ID: "uuid", RemnaSquadUUID: "uuid"}}
	importer := &adminSquadImporter{squads: []UpstreamSquad{{UUID: "uuid", Name: "Upstream Fast"}}}
	service := newAdminServiceForTest(repository, nil, importer, &adminBackupRunner{}, &adminRefunder{})
	profile := &squadprofile.Profile{Type: squadprofile.InternationalNetwork, CountryCode: "US", UpstreamCarriers: []string{"Carrier"}}
	if err := service.DeleteCombo(context.Background(), "admin", "combo-1"); err != nil || repository.deletedComboID != "combo-1" {
		t.Fatalf("DeleteCombo() = %v, ID %q", err, repository.deletedComboID)
	}
	product, err := service.SaveSquadProduct(context.Background(), "admin", database.SquadProductInput{RemnaSquadUUID: " uuid ", Name: "Ignored Local Name", PriceTXBMinor: 10, Profile: profile})
	if err != nil || product.ID != "uuid" || product.Name != "Upstream Fast" || !product.UpstreamPresent ||
		repository.squadInput.RemnaSquadUUID != "uuid" || repository.squadInput.ID != "uuid" || repository.squadInput.Name != "Upstream Fast" || !repository.squadInput.UpstreamPresent {
		t.Fatalf("SaveSquadProduct() = (%+v, %v), input %+v", product, err, repository.squadInput)
	}

	for _, input := range []database.SquadProductInput{
		{Name: "name"},
		{RemnaSquadUUID: "uuid", Name: "name", PriceTXBMinor: -1},
		{RemnaSquadUUID: "uuid", Name: "name", PriceTXBMinor: 1_000_000_000_001},
	} {
		if _, err := service.SaveSquadProduct(context.Background(), "admin", input); err == nil {
			t.Errorf("SaveSquadProduct(%+v) unexpectedly succeeded", input)
		}
	}
	missing := &adminCatalogRepository{}
	if _, err := newAdminServiceForTest(missing, nil, importer, &adminBackupRunner{}, &adminRefunder{}).
		SaveSquadProduct(context.Background(), "admin", database.SquadProductInput{RemnaSquadUUID: "invented", Name: "name", Profile: profile}); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("SaveSquadProduct(invented UUID) = %v, want not found", err)
	}

	repository.deleteComboErr = errors.New("delete failure")
	if err := service.DeleteCombo(context.Background(), "admin", "combo-2"); err == nil {
		t.Fatal("DeleteCombo() ignored repository failure")
	}
	repository.deleteComboErr = nil
	repository.saveSquadErr = errors.New("save failure")
	if _, err := service.SaveSquadProduct(context.Background(), "admin", database.SquadProductInput{RemnaSquadUUID: "uuid", Name: "name", Profile: profile}); err == nil {
		t.Fatal("SaveSquadProduct() ignored repository failure")
	}
	repository.saveSquadErr = nil
	repository.auditErr = errors.New("audit failure")
	if err := service.DeleteCombo(context.Background(), "admin", "combo-3"); err == nil {
		t.Fatal("DeleteCombo() ignored audit failure")
	}
	if _, err := service.SaveSquadProduct(context.Background(), "admin", database.SquadProductInput{RemnaSquadUUID: "uuid", Name: "name", Profile: profile}); err == nil {
		t.Fatal("SaveSquadProduct() ignored audit failure")
	}
}

func TestAdminImportSquads(t *testing.T) {
	t.Parallel()

	upstream := []UpstreamSquad{{UUID: "uuid-1", Name: "One"}, {UUID: "uuid-2", Name: "Two"}}
	repository := &adminCatalogRepository{
		listedSquads: []model.SquadProduct{{ID: "uuid-1", RemnaSquadUUID: "uuid-1", Name: "Stale", Description: "Local", PriceTXBMinor: 25, Visible: true}},
		refreshErr:   errors.New("compatibility refresh must not be called"),
	}
	importer := &adminSquadImporter{squads: upstream}
	service := newAdminServiceForTest(repository, nil, importer, &adminBackupRunner{}, &adminRefunder{})

	products, err := service.ImportSquads(context.Background(), "admin")
	if err != nil || len(products) != 2 {
		t.Fatalf("ImportSquads() = (%+v, %v)", products, err)
	}
	if products[0].ID != "uuid-1" || products[0].Name != "One" || products[0].Description != "Local" || !products[0].Visible || !products[0].UpstreamPresent {
		t.Fatalf("overlaid squad = %+v", products[0])
	}
	if products[1].ID != "uuid-2" || products[1].Name != "Two" || products[1].Visible || !products[1].UpstreamPresent {
		t.Fatalf("virtual squad = %+v", products[1])
	}
	if len(repository.importedSquads) != 0 {
		t.Fatalf("ImportSquads() persisted upstream identities: %+v", repository.importedSquads)
	}
	if !strings.Contains(repository.audits[0].detail, `"count":2`) {
		t.Fatalf("import audit = %+v", repository.audits[0])
	}

	testError := errors.New("failure")
	tests := []struct {
		name       string
		repository *adminCatalogRepository
		importer   *adminSquadImporter
	}{
		{name: "upstream", repository: &adminCatalogRepository{}, importer: &adminSquadImporter{err: testError}},
		{name: "audit", repository: &adminCatalogRepository{auditErr: testError}, importer: &adminSquadImporter{squads: upstream}},
		{name: "list", repository: &adminCatalogRepository{listSquadsErr: testError}, importer: &adminSquadImporter{squads: upstream}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := newAdminServiceForTest(test.repository, nil, test.importer, &adminBackupRunner{}, &adminRefunder{}).ImportSquads(context.Background(), "admin"); err == nil {
				t.Fatal("ImportSquads() unexpectedly succeeded")
			}
		})
	}
}
