package database

const purchaseSquadColumns = `id,user_id,combo_id,entitlement_squad_uuids,combo_squad_uuids,entitlement_addon_squad_uuids`

// selected_purchases supplies the caller's purchase set. This is the shared
// ownership rule for individual entitlement reads and aggregate stock display:
// full entitlement snapshot, then combo/add-on snapshots with legacy fallback,
// plus immutable activity rewards. Never substitute the live combo for a snapshot.
const purchaseSquadMembershipCTE = `owned_squads AS (
	SELECT p.id AS purchase_id,p.user_id,j.value AS group_id
	FROM selected_purchases p,json_each(p.entitlement_squad_uuids) j
	WHERE p.entitlement_squad_uuids IS NOT NULL
	UNION ALL
	SELECT p.id,p.user_id,j.value FROM selected_purchases p
	JOIN combos c ON c.id=p.combo_id,json_each(COALESCE(p.combo_squad_uuids,c.included_squad_uuids)) j
	WHERE p.entitlement_squad_uuids IS NULL
	UNION ALL
	SELECT p.id,p.user_id,a.group_id FROM selected_purchases p
	JOIN purchase_addons a ON a.purchase_id=p.id
	WHERE p.entitlement_squad_uuids IS NULL AND p.entitlement_addon_squad_uuids IS NULL
	UNION ALL
	SELECT p.id,p.user_id,j.value FROM selected_purchases p,json_each(p.entitlement_addon_squad_uuids) j
	WHERE p.entitlement_squad_uuids IS NULL AND p.entitlement_addon_squad_uuids IS NOT NULL
	UNION ALL
	SELECT p.id,p.user_id,a.squad_uuid FROM selected_purchases p
	JOIN activity_reward_squad_access a ON a.purchase_id=p.id
)`
