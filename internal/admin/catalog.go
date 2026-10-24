package admin

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/squadprofile"
	"strings"
)

// SaveCombo validates a complete catalog item.
func (s *Service) SaveCombo(ctx context.Context, actorID string, input database.ComboInput) (model.Combo, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || input.PriceTXBMinor < 0 || input.PriceTXBMinor > 1_000_000_000_000 || input.ValidityDays < 1 || input.ValidityDays > 3650 || input.TrafficLimitBytes <= 0 ||
		input.RolloverMinRemainingBPS < 0 || input.RolloverMinRemainingBPS > 10_000 {
		return model.Combo{}, errors.New("invalid combo fields")
	}
	if input.ResetStrategy != "DAY" && input.ResetStrategy != "WEEK" && input.ResetStrategy != "MONTH_ROLLING" {
		return model.Combo{}, errors.New("invalid reset strategy")
	}
	live, err := s.liveSquads(ctx)
	if err != nil {
		return model.Combo{}, err
	}
	for _, squadUUID := range input.SquadProductIDs {
		if _, exists := live[strings.TrimSpace(squadUUID)]; !exists {
			return model.Combo{}, database.ErrNotFound
		}
	}
	combo, err := s.repository.SaveCombo(ctx, input)
	if err != nil {
		return model.Combo{}, err
	}
	if err := s.audit(ctx, actorID, "combo.save", "combo", combo.ID, map[string]any{"name": combo.Name}); err != nil {
		return model.Combo{}, err
	}
	return combo, nil
}

// DeleteCombo hides the selected combo.
func (s *Service) DeleteCombo(ctx context.Context, actorID, comboID string) error {
	if err := s.repository.DeleteCombo(ctx, comboID); err != nil {
		return err
	}
	return s.audit(ctx, actorID, "combo.hide", "combo", comboID, nil)
}

// SaveSquadProduct validates local merchandising data.
func (s *Service) SaveSquadProduct(ctx context.Context, actorID string, input database.SquadProductInput) (model.SquadProduct, error) {
	input.GroupID = strings.TrimSpace(input.GroupID)
	if input.BillingMode == "" {
		input.BillingMode = "combo"
	}
	if (input.BillingMode != "combo" && input.BillingMode != "payg") || input.PAYGPriceTXBMinor < 0 || input.PAYGPriceTXBMinor > 1_000_000_000_000 || (input.BillingMode == "payg" && input.PAYGPriceTXBMinor == 0) {
		return model.SquadProduct{}, errors.New("invalid PAYG price or billing mode")
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.GroupID == "" || input.PriceTXBMinor < 0 || input.PriceTXBMinor > 1_000_000_000_000 || (input.StockLimit != nil && (*input.StockLimit < 0 || *input.StockLimit > 1_000_000_000)) {
		return model.SquadProduct{}, errors.New("invalid squad product")
	}
	profile, profileErr := squadprofile.Normalize(input.Profile)
	if profileErr != nil {
		return model.SquadProduct{}, profileErr
	}
	input.Profile = profile
	live, err := s.liveSquads(ctx)
	if err != nil {
		return model.SquadProduct{}, err
	}
	upstreamName, exists := live[input.GroupID]
	if !exists {
		return model.SquadProduct{}, database.ErrNotFound
	}
	input.ID = input.GroupID
	input.Name = upstreamName
	input.UpstreamPresent = true
	product, err := s.repository.SaveSquadProduct(ctx, input)
	if err != nil {
		return model.SquadProduct{}, err
	}
	product.Name = upstreamName
	product.UpstreamPresent = true
	if err := s.audit(ctx, actorID, "squad_product.save", "squad_product", product.ID, map[string]any{"name": upstreamName}); err != nil {
		return model.SquadProduct{}, err
	}
	return product, nil
}

// ImportSquads now returns a live upstream list overlaid with the sparse local
// overrides. The retained import call is an audited compatibility endpoint and
// no longer stores a second catalog identity table.
func (s *Service) ImportSquads(ctx context.Context, actorID string) ([]model.SquadProduct, error) {
	upstream, err := s.importer.ListInternalSquads(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.audit(ctx, actorID, "squads.import", "catalog", "squads", map[string]any{"count": len(upstream)}); err != nil {
		return nil, err
	}
	return s.Squads(ctx)
}

// Squads builds the admin catalog from live Proxy identities plus sparse
// local merchandising overrides.
func (s *Service) Squads(ctx context.Context) ([]model.SquadProduct, error) {
	upstream, err := s.importer.ListInternalSquads(ctx)
	if err != nil {
		return nil, err
	}
	overrides, err := s.repository.ListSquadProducts(ctx, false)
	if err != nil {
		return nil, err
	}
	overrideByUUID := make(map[string]model.SquadProduct, len(overrides))
	for _, override := range overrides {
		overrideByUUID[override.GroupID] = override
	}
	products := make([]model.SquadProduct, 0, len(upstream))
	for _, squad := range upstream {
		product, exists := overrideByUUID[squad.UUID]
		if !exists {
			product = model.SquadProduct{ID: squad.UUID, GroupID: squad.UUID, BillingMode: "combo", Visible: false, Price: model.TXBMoney(0),
				AccessibleNodes: []model.CatalogNode{}}
		}
		if product.AccessibleNodes == nil {
			product.AccessibleNodes = []model.CatalogNode{}
		}
		product.ID = squad.UUID
		product.GroupID = squad.UUID
		product.Name = squad.Name
		product.UpstreamPresent = true
		products = append(products, product)
	}
	return products, nil
}

func (s *Service) liveSquads(ctx context.Context) (map[string]string, error) {
	upstream, err := s.importer.ListInternalSquads(ctx)
	if err != nil {
		return nil, err
	}
	live := make(map[string]string, len(upstream))
	for _, squad := range upstream {
		live[strings.TrimSpace(squad.UUID)] = strings.TrimSpace(squad.Name)
	}
	return live, nil
}
