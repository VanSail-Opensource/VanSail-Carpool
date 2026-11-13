package admin

import (
	"context"
	"encoding/json"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func (r *adminCatalogRepository) SaveAdminCombo(_ context.Context, _ string, input database.ComboInput) (model.Combo, error) {
	if r.saveComboErr != nil {
		return model.Combo{}, r.saveComboErr
	}
	if r.auditErr != nil {
		return model.Combo{}, r.auditErr
	}
	r.comboInput = input
	r.audits = append(r.audits, adminAuditCall{action: "combo.save", targetType: "combo", targetID: r.savedCombo.ID})
	return r.savedCombo, nil
}

func (r *adminCatalogRepository) DeleteAdminCombo(_ context.Context, _ string, id string) error {
	if r.deleteComboErr != nil {
		return r.deleteComboErr
	}
	if r.auditErr != nil {
		return r.auditErr
	}
	r.deletedComboID = id
	r.audits = append(r.audits, adminAuditCall{action: "combo.hide", targetType: "combo", targetID: id})
	return nil
}

func (r *adminCatalogRepository) SaveAdminSquadProduct(_ context.Context, _ string, input database.SquadProductInput) (model.SquadProduct, error) {
	if r.saveSquadErr != nil {
		return model.SquadProduct{}, r.saveSquadErr
	}
	if r.auditErr != nil {
		return model.SquadProduct{}, r.auditErr
	}
	detail, err := json.Marshal(map[string]string{"name": input.Name})
	if err != nil {
		return model.SquadProduct{}, err
	}
	r.squadInput = input
	r.savedSquadInputs = append(r.savedSquadInputs, input)
	r.audits = append(r.audits, adminAuditCall{action: "squad_product.save", targetType: "squad_product", targetID: input.GroupID, detail: string(detail)})
	if r.savedSquad.ID != "" {
		return r.savedSquad, nil
	}
	return model.SquadProduct{ID: input.GroupID, Name: input.Name}, nil
}
