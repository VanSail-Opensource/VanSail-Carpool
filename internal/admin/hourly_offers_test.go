package admin

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

type hourlyOfferTestRepository struct {
	*adminCatalogRepository
	offer model.HourlyOffer
	saves int
}

func (r *hourlyOfferTestRepository) HourlyOffer(context.Context, string) (model.HourlyOffer, error) {
	if r.offer.Revision == 0 {
		return model.HourlyOffer{}, database.ErrNotFound
	}
	return r.offer, nil
}

func (r *hourlyOfferTestRepository) ListHourlyOffers(context.Context) ([]model.HourlyOffer, error) {
	if r.offer.Revision == 0 {
		return []model.HourlyOffer{}, nil
	}
	return []model.HourlyOffer{r.offer}, nil
}

func (r *hourlyOfferTestRepository) SaveHourlyOffer(_ context.Context, _ string, next model.HourlyOffer, expected int64) (model.HourlyOffer, error) {
	if r.offer.Revision != expected {
		return model.HourlyOffer{}, database.ErrConflict
	}
	r.saves++
	next.Revision = expected + 1
	next.HourlyPrice = model.TXBMoney(next.HourlyPriceTXBMinor)
	r.offer = next
	return next, nil
}

func TestHourlyOffersStaySeparateAndDoNotRequireSalesToConfigure(t *testing.T) {
	ctx := context.Background()
	settingsRepo := &hourlyPolicyTestRepository{adminSettingsRepository: newAdminSettingsRepository()}
	repo := &hourlyOfferTestRepository{adminCatalogRepository: &adminCatalogRepository{}}
	service := NewService(repo, NewSettingsService(settingsRepo, testVault(t)), nil, nil, nil)
	offers, err := service.HourlyOffers(ctx)
	if err != nil || len(offers) != 0 || settingsRepo.saves != 0 {
		t.Fatal("reading offers seeded pricing or enabled sales", err)
	}
	next := model.HourlyOffer{GroupID: "independent-group", Enabled: true, HourlyPriceTXBMinor: 123, RequiredHours: 2}
	saved, err := service.SaveHourlyOffer(ctx, "admin", next, 0)
	if err != nil || saved.Revision != 1 || saved.HourlyPrice.Minor != "123" || saved.RequiredHours != 2 || repo.saves != 1 || len(repo.savedSquadInputs) != 0 {
		t.Fatal("hourly configuration mutated legacy merchandising", err)
	}
	read, err := service.HourlyOffer(ctx, next.GroupID)
	if err != nil || read != saved {
		t.Fatal("explicit offer did not round trip", err)
	}
	next.Enabled, next.HourlyPriceTXBMinor = false, 0
	updated, err := service.SaveHourlyOffer(ctx, "admin", next, saved.Revision)
	if err != nil || updated.Enabled || updated.HourlyPrice.Minor != "0" || updated.Revision != 2 {
		t.Fatal("disabling an offer did not retain explicit revision semantics", err)
	}
	if _, err := service.SaveHourlyOffer(ctx, "admin", next, saved.Revision); !errors.Is(err, database.ErrConflict) || repo.saves != 2 {
		t.Fatal("stale offer edit reached persistence", err)
	}
}

func TestHourlyOfferValidationRejectsUnavailableAndOverflowingPrices(t *testing.T) {
	ctx := context.Background()
	settingsRepo := &hourlyPolicyTestRepository{adminSettingsRepository: newAdminSettingsRepository()}
	repo := &hourlyOfferTestRepository{adminCatalogRepository: &adminCatalogRepository{}}
	service := NewService(repo, NewSettingsService(settingsRepo, testVault(t)), nil, nil, nil)
	for _, next := range []model.HourlyOffer{
		{GroupID: "group", Enabled: true},
		{GroupID: "group", HourlyPriceTXBMinor: -1},
		{GroupID: "group", Enabled: true, HourlyPriceTXBMinor: math.MaxInt64},
		{GroupID: "group", Enabled: true, HourlyPriceTXBMinor: 1, RequiredHours: -1},
		{GroupID: "group", Enabled: true, HourlyPriceTXBMinor: 1, RequiredHours: model.MaxHourlyRequiredHours + 1},
		{GroupID: " ", Enabled: true, HourlyPriceTXBMinor: 1},
	} {
		if _, err := service.SaveHourlyOffer(ctx, "admin", next, 0); !errors.Is(err, model.ErrHourlyPolicyInvalid) || repo.saves != 0 {
			t.Fatal("invalid offer reached persistence", err)
		}
	}
	valid := model.HourlyOffer{GroupID: "group", Enabled: true, HourlyPriceTXBMinor: 1, RequiredHours: model.MaxHourlyRequiredHours}
	if _, err := service.SaveHourlyOffer(ctx, "admin", valid, -1); !errors.Is(err, model.ErrHourlyPolicyInvalid) || repo.saves != 0 {
		t.Fatal("negative revision reached persistence", err)
	}
}
