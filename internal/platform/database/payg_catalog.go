package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// Filter only future sales. Existing purchaseSquads never filters by live mode.
func comboSquadsTx(ctx context.Context, tx *sql.Tx, squads []model.SquadProduct) ([]model.SquadProduct, error) {
	out := make([]model.SquadProduct, 0, len(squads))
	for _, squad := range squads {
		var mode string
		err := tx.QueryRowContext(ctx, `SELECT billing_mode FROM squad_product_overrides WHERE group_id=?`, squad.GroupID).Scan(&mode)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		if mode != "payg" {
			out = append(out, squad)
		}
	}
	return out, nil
}

func squadJSON(squads []model.SquadProduct) string {
	ids := make([]string, 0, len(squads))
	for _, squad := range squads {
		ids = append(ids, squad.GroupID)
	}
	raw, _ := json.Marshal(uniqueSorted(ids))
	return string(raw)
}

// snapshotComboSquadsTx retains a base-only snapshot: ordinary add-ons remain
// editable and custom full-entitlement overrides retain their existing meaning.
func snapshotComboSquadsTx(ctx context.Context, tx *sql.Tx, purchase string, squads []model.SquadProduct) error {
	_, err := tx.ExecContext(ctx, `UPDATE purchases SET combo_squad_uuids=CASE WHEN (SELECT included_squad_uuids FROM combos WHERE combos.id=purchases.combo_id)=? THEN NULL ELSE ? END WHERE id=?`, squadJSON(squads), squadJSON(squads), purchase)
	return err
}

func renewalComboTx(ctx context.Context, tx *sql.Tx, purchase string, combo model.Combo) (model.Combo, error) {
	var custom sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT entitlement_squad_uuids FROM purchases WHERE id=?`, purchase).Scan(&custom); err != nil {
		return combo, err
	}
	if custom.Valid {
		var ids []string
		if err := json.Unmarshal([]byte(custom.String), &ids); err != nil {
			return combo, err
		}
		combo.IncludedSquads = []model.SquadProduct{}
		for _, id := range uniqueSorted(ids) {
			combo.IncludedSquads = append(combo.IncludedSquads, model.SquadProduct{ID: id, GroupID: id})
		}
	}
	var err error
	combo.IncludedSquads, err = comboSquadsTx(ctx, tx, combo.IncludedSquads)
	return combo, err
}
