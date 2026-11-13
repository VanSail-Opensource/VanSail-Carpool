package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type distributionFact struct {
	comboID, comboName, squadUUID, squadName string
	count                                    float64
}

// Statistics use the same retained entitlement graph as stock and access reads.
// A live catalog edit cannot replace stored selections or hide activity rewards.
var activeEntitlementAssignments = `WITH selected_purchases AS (
 SELECT ` + purchaseSquadColumns + ` FROM purchases
 WHERE EXISTS(SELECT 1 FROM users member WHERE member.id=purchases.user_id AND member.role='user')
 AND status IN ('active','activating') AND ` + purchaseTimestampOrderSQL("valid_from") + `<=$1 AND ` + purchaseTimestampOrderSQL("valid_until") + `>$2
), ` + purchaseSquadMembershipCTE + `, active AS (
 SELECT p.id purchase_id,p.user_id,p.combo_id,
 COALESCE(p.combo_squad_uuids,c.included_squad_uuids) included_squad_uuids
 FROM selected_purchases p JOIN combos c ON c.id=p.combo_id
), assignments AS (
 SELECT DISTINCT a.purchase_id,a.user_id,a.combo_id,o.group_id squad_uuid
 FROM active a JOIN owned_squads o ON o.purchase_id=a.purchase_id
)
`

func activeCatalogStatisticsTx(ctx context.Context, tx *sql.Tx, now time.Time) ([]model.NamedShare, float64, []model.NormalizedDistribution, []model.NormalizedDistribution, error) {
	comboRows, err := tx.QueryContext(ctx, `SELECT c.id,c.name,COUNT(DISTINCT p.user_id) FROM purchases p JOIN combos c ON c.id=p.combo_id
		JOIN users member ON member.id=p.user_id WHERE member.role='user'
		AND p.status IN ('active','activating') AND `+purchaseTimestampOrderSQL("p.valid_from")+`<=$1 AND `+purchaseTimestampOrderSQL("p.valid_until")+`>$2 GROUP BY c.id,c.name ORDER BY c.name`, purchaseTimestampOrder(now), purchaseTimestampOrder(now))
	if err != nil {
		return nil, 0, nil, nil, err
	}
	shares := make([]model.NamedShare, 0)
	for comboRows.Next() {
		var item model.NamedShare
		if err := comboRows.Scan(&item.ID, &item.Label, &item.Value); err != nil {
			_ = comboRows.Close()
			return nil, 0, nil, nil, err
		}
		shares = append(shares, item)
	}
	if err := errors.Join(comboRows.Err(), comboRows.Close()); err != nil {
		return nil, 0, nil, nil, err
	}
	var optional float64
	optionalQuery := activeEntitlementAssignments + `SELECT COALESCE(AVG(optional_count),0) FROM (
		SELECT active.user_id,COUNT(DISTINCT CASE WHEN assignments.squad_uuid IS NOT NULL AND NOT EXISTS (
			SELECT 1 FROM jsonb_array_elements_text((active.included_squad_uuids)::jsonb)
			WHERE value=assignments.squad_uuid)
			THEN assignments.squad_uuid END) optional_count
		FROM active LEFT JOIN assignments ON assignments.purchase_id=active.purchase_id GROUP BY active.user_id)`
	if err := tx.QueryRowContext(ctx, optionalQuery, purchaseTimestampOrder(now), purchaseTimestampOrder(now)).Scan(&optional); err != nil {
		return nil, 0, nil, nil, err
	}
	facts, err := activeDistributionFactsTx(ctx, tx, now)
	if err != nil {
		return nil, 0, nil, nil, err
	}
	return shares, optional, normalizeDistributions(facts, true), normalizeDistributions(facts, false), nil
}

func activeDistributionFactsTx(ctx context.Context, tx *sql.Tx, now time.Time) ([]distributionFact, error) {
	rows, err := tx.QueryContext(ctx, activeEntitlementAssignments+`
	SELECT c.id,c.name,assignments.squad_uuid,assignments.squad_uuid,COUNT(DISTINCT assignments.user_id)
	FROM assignments JOIN combos c ON c.id=assignments.combo_id
	GROUP BY c.id,c.name,assignments.squad_uuid ORDER BY c.name,assignments.squad_uuid`, purchaseTimestampOrder(now), purchaseTimestampOrder(now))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := make([]distributionFact, 0)
	for rows.Next() {
		var fact distributionFact
		if err := rows.Scan(&fact.comboID, &fact.comboName, &fact.squadUUID, &fact.squadName, &fact.count); err != nil {
			return nil, err
		}
		result = append(result, fact)
	}
	return result, rows.Err()
}

func normalizeDistributions(facts []distributionFact, byCombo bool) []model.NormalizedDistribution {
	groups := make(map[string]*model.NormalizedDistribution)
	order := make([]string, 0)
	for _, fact := range facts {
		groupID, groupLabel, segmentID, segmentLabel := fact.comboID, fact.comboName, fact.squadUUID, fact.squadName
		if !byCombo {
			groupID, groupLabel, segmentID, segmentLabel = fact.squadUUID, fact.squadName, fact.comboID, fact.comboName
		}
		group := groups[groupID]
		if group == nil {
			group = &model.NormalizedDistribution{ID: groupID, Label: groupLabel}
			groups[groupID], order = group, append(order, groupID)
		}
		group.Segments = append(group.Segments, model.NamedShare{ID: segmentID, Label: segmentLabel, Value: fact.count})
	}
	result := make([]model.NormalizedDistribution, 0, len(order))
	for _, id := range order {
		group := groups[id]
		var total float64
		for _, segment := range group.Segments {
			total += segment.Value
		}
		if total > 0 {
			for index := range group.Segments {
				group.Segments[index].Value = group.Segments[index].Value * 100 / total
			}
		}
		result = append(result, *group)
	}
	return result
}
