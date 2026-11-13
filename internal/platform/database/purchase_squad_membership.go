package database

import (
	"context"
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

const purchaseSquadColumns = `id,user_id,combo_id,entitlement_squad_uuids,combo_squad_uuids,entitlement_addon_squad_uuids`

// selected_purchases supplies the caller's purchase set. This is the shared
// ownership rule for individual entitlement reads and aggregate stock display:
// full entitlement snapshot, then combo/add-on snapshots with legacy fallback,
// plus immutable activity rewards. Never substitute the live combo for a snapshot.
const purchaseSquadMembershipCTE = `owned_squads AS (
	SELECT p.id AS purchase_id,p.user_id,j.value AS group_id
	FROM selected_purchases p,jsonb_array_elements_text((p.entitlement_squad_uuids)::jsonb) j
	WHERE p.entitlement_squad_uuids IS NOT NULL
	UNION ALL
	SELECT p.id,p.user_id,j.value FROM selected_purchases p
	JOIN combos c ON c.id=p.combo_id,jsonb_array_elements_text((COALESCE(p.combo_squad_uuids,c.included_squad_uuids))::jsonb) j
	WHERE p.entitlement_squad_uuids IS NULL
	UNION ALL
	SELECT p.id,p.user_id,a.group_id FROM selected_purchases p
	JOIN purchase_addons a ON a.purchase_id=p.id
	WHERE p.entitlement_squad_uuids IS NULL AND p.entitlement_addon_squad_uuids IS NULL
	UNION ALL
	SELECT p.id,p.user_id,j.value FROM selected_purchases p,jsonb_array_elements_text((p.entitlement_addon_squad_uuids)::jsonb) j
	WHERE p.entitlement_squad_uuids IS NULL AND p.entitlement_addon_squad_uuids IS NOT NULL
	UNION ALL
	SELECT p.id,p.user_id,a.squad_uuid FROM selected_purchases p
	JOIN activity_reward_squad_access a ON a.purchase_id=p.id
)`

// Hydrate complete purchase pages with bounded set queries rather than one
// database round trip per purchase. The same ownership CTE defines individual
// reads, frozen snapshots, legacy paid fallback and immutable activity rewards.
func hydratePurchaseSquads(ctx context.Context, queryer purchaseSquadQueryer, purchases []model.Purchase) error {
	const batchSize = 256
	for start := 0; start < len(purchases); start += batchSize {
		batch := purchases[start:min(start+batchSize, len(purchases))]
		ids := make([]string, len(batch))
		for index := range batch {
			ids[index] = batch[index].ID
		}
		rows, err := queryer.QueryContext(ctx, `WITH selected_purchases AS (SELECT `+purchaseSquadColumns+` FROM purchases WHERE id=ANY($1)), `+purchaseSquadMembershipCTE+`
			SELECT DISTINCT purchase_id,group_id FROM owned_squads ORDER BY purchase_id,group_id`, ids)
		if err != nil {
			return fmt.Errorf("load purchase squad batch: %w", err)
		}
		groups := make(map[string][]string, len(batch))
		for rows.Next() {
			var purchase, group string
			if err := rows.Scan(&purchase, &group); err != nil {
				rows.Close()
				return err
			}
			groups[purchase] = append(groups[purchase], group)
		}
		if err := finishCompileRows(rows); err != nil {
			return err
		}
		for index := range batch {
			batch[index].SquadUUIDs = groups[batch[index].ID]
			if batch[index].SquadUUIDs == nil {
				batch[index].SquadUUIDs = []string{}
			}
		}
	}
	return nil
}
