package database

// comboControlConflictSQL shares the same in-flight authority gate between
// transactional admission and bounded repair candidate selection. Only fixed
// internal column or placeholder expressions may enter this SQL fragment.
func comboControlConflictSQL(ownerExpression string) string {
	return `(EXISTS(SELECT 1 FROM provider_operations operation
 LEFT JOIN provider_operation_items item ON item.operation_id=operation.id
 WHERE operation.kind IN ('purchase_traffic_reset','purchase_refund','member_squad_switch','member_early_activation',
 'subscription_revoke','xnode_routing','xnode_diy','admin_entitlement_edit','admin_entitlement_refund','admin_combo_replacement',
 'admin_bulk_extension','node_compensation','admin_temporary_ban','admin_temporary_unban')
 AND operation.status IN ('queued','processing','pending_review','partial') AND
 (operation.owner_user_id=` + ownerExpression + ` OR (item.target_type='user' AND item.target_id=` + ownerExpression + `) OR
 (item.target_type='purchase' AND item.target_id IN (SELECT id FROM purchases WHERE user_id=` + ownerExpression + `))))
 OR EXISTS(SELECT 1 FROM outbox_jobs job WHERE job.status='processing' AND
 (((CASE ((CASE WHEN COALESCE((job.payload) IS JSON,false) THEN job.payload ELSE '{}' END)::jsonb #> '{userId}')::text WHEN 'true' THEN '1' WHEN 'false' THEN '0' ELSE (CASE WHEN COALESCE((job.payload) IS JSON,false) THEN job.payload ELSE '{}' END)::jsonb #>> '{userId}' END)=` + ownerExpression + ` AND job.kind='xnode_sync_user') OR
 (job.kind IN ('xnode_apply_entitlement','xnode_prepare_continuity','purchase_expire') AND
 (CASE ((CASE WHEN COALESCE((job.payload) IS JSON,false) THEN job.payload ELSE '{}' END)::jsonb #> '{purchaseId}')::text WHEN 'true' THEN '1' WHEN 'false' THEN '0' ELSE (CASE WHEN COALESCE((job.payload) IS JSON,false) THEN job.payload ELSE '{}' END)::jsonb #>> '{purchaseId}' END) IN (SELECT id FROM purchases WHERE user_id=` + ownerExpression + `))))
 OR EXISTS(SELECT 1 FROM admin_temporary_bans WHERE user_id=` + ownerExpression + ` AND restored_at IS NULL))`
}
