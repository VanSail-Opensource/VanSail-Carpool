package database

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type distributionFact struct {
	comboID, comboName, squadUUID, squadName string
	count                                    float64
}

const activeEntitlementAssignments = `WITH active AS (
	SELECT p.id purchase_id,p.user_id,p.combo_id,p.entitlement_squad_uuids,p.entitlement_addon_squad_uuids
	FROM purchases p JOIN users member ON member.id=p.user_id
	WHERE member.role='user' AND p.status IN ('active','activating') AND p.valid_from<=? AND p.valid_until>?),
assignments AS (
	SELECT active.purchase_id,active.user_id,active.combo_id,value squad_uuid
	FROM active,json_each(active.entitlement_squad_uuids) WHERE active.entitlement_squad_uuids IS NOT NULL
	UNION SELECT active.purchase_id,active.user_id,active.combo_id,value FROM active
		JOIN combos c ON c.id=active.combo_id,json_each(c.included_squad_uuids) WHERE active.entitlement_squad_uuids IS NULL
	UNION SELECT active.purchase_id,active.user_id,active.combo_id,a.remna_squad_uuid FROM active
		JOIN purchase_addons a ON a.purchase_id=active.purchase_id
		WHERE active.entitlement_squad_uuids IS NULL AND active.entitlement_addon_squad_uuids IS NULL
	UNION SELECT active.purchase_id,active.user_id,active.combo_id,value FROM active,
		json_each(active.entitlement_addon_squad_uuids)
		WHERE active.entitlement_squad_uuids IS NULL AND active.entitlement_addon_squad_uuids IS NOT NULL)
`

func (s *Store) activeCatalogStatistics(ctx context.Context, now time.Time) ([]model.NamedShare, float64, []model.NormalizedDistribution, []model.NormalizedDistribution, error) {
	comboRows, err := s.db.QueryContext(ctx, `SELECT c.id,c.name,COUNT(DISTINCT p.user_id) FROM purchases p JOIN combos c ON c.id=p.combo_id
		JOIN users member ON member.id=p.user_id WHERE member.role='user'
		AND p.status IN ('active','activating') AND p.valid_from<=? AND p.valid_until>? GROUP BY c.id,c.name ORDER BY c.name`, stamp(now), stamp(now))
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
	if err := comboRows.Close(); err != nil {
		return nil, 0, nil, nil, err
	}
	var optional float64
	optionalQuery := activeEntitlementAssignments + `SELECT COALESCE(AVG(optional_count),0) FROM (
		SELECT active.user_id,COUNT(DISTINCT CASE WHEN assignments.squad_uuid IS NOT NULL AND NOT EXISTS (
			SELECT 1 FROM combos included_combo,json_each(included_combo.included_squad_uuids)
			WHERE included_combo.id=assignments.combo_id AND value=assignments.squad_uuid)
			THEN assignments.squad_uuid END) optional_count
		FROM active LEFT JOIN assignments ON assignments.purchase_id=active.purchase_id GROUP BY active.user_id)`
	if err := s.db.QueryRowContext(ctx, optionalQuery, stamp(now), stamp(now)).Scan(&optional); err != nil {
		return nil, 0, nil, nil, err
	}
	facts, err := s.activeDistributionFacts(ctx, now)
	if err != nil {
		return nil, 0, nil, nil, err
	}
	return shares, optional, normalizeDistributions(facts, true), normalizeDistributions(facts, false), nil
}

func (s *Store) activeDistributionFacts(ctx context.Context, now time.Time) ([]distributionFact, error) {
	rows, err := s.db.QueryContext(ctx, activeEntitlementAssignments+`
	SELECT c.id,c.name,assignments.squad_uuid,assignments.squad_uuid,COUNT(DISTINCT assignments.user_id)
	FROM assignments JOIN combos c ON c.id=assignments.combo_id
	GROUP BY c.id,c.name,assignments.squad_uuid ORDER BY c.name,assignments.squad_uuid`, stamp(now), stamp(now))
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
