package app

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/catalog"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/telegrampm"
)

type pmProfileReader struct {
	users   *admin.UserWorkflows
	catalog *catalog.Service
	store   *database.Store
}

func (r pmProfileReader) PMProfileFacts(ctx context.Context, userID string) (telegrampm.ProfileFacts, error) {
	user, err := r.store.UserByID(ctx, userID)
	if err != nil {
		return telegrampm.ProfileFacts{}, err
	}
	balance, err := r.store.Balance(ctx, userID)
	if err != nil {
		return telegrampm.ProfileFacts{}, err
	}
	facts := telegrampm.ProfileFacts{OnboardingState: user.OnboardingState, Balance: balance, RemnaUsername: user.AccountName()}
	if user.OnboardingState != "complete" {
		return facts, nil
	}
	active, _, err := r.store.ActiveAndQueuedPurchases(ctx, userID, time.Now().UTC())
	if err != nil {
		return telegrampm.ProfileFacts{}, err
	}
	if active == nil {
		return facts, nil
	}
	purchase := *active
	facts.ActiveCombo = &purchase
	squadNames := make(map[string]string, len(purchase.SquadUUIDs))
	for _, squadID := range purchase.SquadUUIDs {
		product, lookupErr := r.store.SquadProductByRemnaUUID(ctx, squadID)
		if lookupErr == nil && strings.TrimSpace(product.Name) != "" {
			squadNames[squadID] = product.Name
		}
	}
	if len(squadNames) < len(purchase.SquadUUIDs) {
		liveCatalog, catalogErr := r.catalog.Catalog(ctx)
		if catalogErr == nil {
			for _, addon := range liveCatalog.Addons {
				if _, selected := squadNames[addon.RemnaSquadUUID]; !selected && addon.Name != "" {
					squadNames[addon.RemnaSquadUUID] = addon.Name
				}
			}
		}
	}
	for _, squadID := range purchase.SquadUUIDs {
		if name := squadNames[squadID]; name != "" {
			facts.SquadNames = append(facts.SquadNames, name)
		} else {
			facts.SquadNames = append(facts.SquadNames, squadID)
		}
	}
	quote, quoteErr := r.users.RefundEntitlementQuote(ctx, userID, purchase.ID)
	if quoteErr == nil && quote.AllocatedTrafficBytes != nil && quote.UsedTrafficBytes != nil {
		allocated, allocatedErr := strconv.ParseInt(*quote.AllocatedTrafficBytes, 10, 64)
		used, usedErr := strconv.ParseInt(*quote.UsedTrafficBytes, 10, 64)
		if allocatedErr == nil && usedErr == nil && allocated > 0 && used >= 0 {
			facts.AllocatedBytes, facts.UsedBytes = &allocated, &used
		}
	}
	projection, projectionErr := r.catalog.RolloverProjection(ctx, user, purchase.ID)
	if projectionErr == nil {
		facts.Rollover = &projection
	}
	return facts, nil
}
