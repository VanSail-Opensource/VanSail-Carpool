package database

import (
	"context"
	"testing"
)

func customComboRecoverySnapshot(t *testing.T, store *Store, purchaseID string) string {
	t.Helper()
	var snapshot string
	err := store.DB().QueryRowContext(context.Background(), `SELECT json_object(
		'combo',combo_id,'squads',entitlement_squad_uuids,'addons',entitlement_addon_squad_uuids,
		'traffic',entitlement_traffic_limit_bytes,'reset',entitlement_reset_strategy,
		'price',reward_renewal_price_minor,'rollover',reward_rollover_min_remaining_bps,
		'renewTraffic',reward_renewal_traffic_limit_bytes,'trafficRenews',reward_traffic_renewal,
		'autoRenew',auto_renew_enabled,'status',status,'from',valid_from,'until',valid_until,
		'charged',charged_txb_minor,'updated',updated_at) FROM purchases WHERE id=$1`, purchaseID).Scan(&snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}
