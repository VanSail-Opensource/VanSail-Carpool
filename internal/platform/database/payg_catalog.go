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
		err := tx.QueryRowContext(ctx, `SELECT billing_mode FROM squad_product_overrides WHERE group_id=$1`, squad.GroupID).Scan(&mode)
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
	_, err := tx.ExecContext(ctx, `UPDATE purchases SET combo_squad_uuids=CASE WHEN (SELECT included_squad_uuids FROM combos WHERE combos.id=purchases.combo_id)=$1 THEN NULL ELSE $2 END WHERE id=$3`, squadJSON(squads), squadJSON(squads), purchase)
	return err
}

// ownedRenewalComboTx resolves the combo an owned renewal actually provisions.
// The combo row must exist, but a hidden or inactive row stays renewable:
// visibility, billing mode and bundle edits are merchandising state for new
// sales and must not rewrite an owned renewal. A missing row fails closed with
// ErrNotFound, which callers report as an unavailable combo.
func ownedRenewalComboTx(ctx context.Context, tx *sql.Tx, purchase string) (model.Combo, error) {
	var comboID string
	if err := tx.QueryRowContext(ctx, `SELECT combo_id FROM purchases WHERE id=$1`, purchase).Scan(&comboID); err != nil {
		return model.Combo{}, err
	}
	combo, err := comboByIDTx(ctx, tx, comboID, false)
	if err != nil {
		return model.Combo{}, err
	}
	return renewalComboTx(ctx, tx, purchase, combo)
}

// renewalComboReader is the single-statement read surface shared by the
// transactional plan/quote resolution and the plain catalog resolver.
type renewalComboReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// renewalComboTx applies a purchase's persisted owned squad selection. A full
// manual/draw override wins; otherwise the purchase's recorded base snapshot
// wins over the live bundle so a later bundle edit cannot rewrite the owned
// renewal. Both keep every included squad regardless of the product's current
// billing mode; comboSquadsTx stays sale-only. A NULL snapshot means the
// purchase selection matched the live bundle, so the live bundle remains the
// correct legacy meaning.
func renewalComboTx(ctx context.Context, queryer renewalComboReader, purchase string, combo model.Combo) (model.Combo, error) {
	var override, snapshot sql.NullString
	if err := queryer.QueryRowContext(ctx, `SELECT entitlement_squad_uuids,combo_squad_uuids FROM purchases WHERE id=$1`, purchase).Scan(&override, &snapshot); err != nil {
		return combo, err
	}
	encoded := override
	if !encoded.Valid {
		encoded = snapshot
	}
	if !encoded.Valid {
		return combo, nil
	}
	var ids []string
	if err := json.Unmarshal([]byte(encoded.String), &ids); err != nil {
		return combo, err
	}
	combo.IncludedSquads = make([]model.SquadProduct, 0, len(ids))
	for _, id := range uniqueSorted(ids) {
		combo.IncludedSquads = append(combo.IncludedSquads, model.SquadProduct{ID: id, GroupID: id})
	}
	return combo, nil
}
