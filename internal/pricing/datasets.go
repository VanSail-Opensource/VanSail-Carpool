package pricing

// Dataset is an explicit secret-free analytical source, never SELECT *.
// Scope expressions refer to aliases in Query. Shared definitions accompany any
// filtered export. No descriptor includes credentials, addresses or sessions.
type Dataset struct {
	Name, Query, Date, User, Node, Product string
	Shared, Users, Financial               bool
	RetentionDays                          int
}

// Datasets inventories every retained pricing input and generated record.
func Datasets() []Dataset {
	return []Dataset{
		{Name: "raw/users", Query: `SELECT id,created_at,onboarding_state FROM users`, Date: "created_at", User: "id", Users: true},
		{Name: "raw/machines", Query: `SELECT id,name,provider_name,enabled,created_at FROM xnode_nodes`, Date: "created_at", Node: "id", Shared: true},
		{Name: "raw/node_traffic", Query: `SELECT d.proxy_user_id,COALESCE(i.user_id,a.user_id) AS user_id,d.node_id,d.period_id,d.date,d.upload_bytes,d.download_bytes,d.weighted_bytes,d.weight_remainder FROM xnode_daily_usage d JOIN xnode_identities i ON i.proxy_user_id=d.proxy_user_id LEFT JOIN xnode_payg_accounts a ON a.proxy_user_id=d.proxy_user_id`, Date: "d.date", User: "COALESCE(i.user_id,a.user_id)", Node: "d.node_id", Users: true, RetentionDays: 60},
		{Name: "raw/connectivity", Query: `SELECT h.id,h.host_uuid,e.node_id,h.status,h.latency_ms,h.error_code,h.started_at,h.finished_at FROM host_connectivity_attempts h LEFT JOIN xnode_endpoints e ON e.id=h.host_uuid`, Date: "h.started_at", Node: "e.node_id", RetentionDays: 1},
		{Name: "raw/wallet_events", Query: `SELECT id,user_id,delta_txb_minor,kind,reference_id,created_at FROM ledger_entries`, Date: "created_at", User: "user_id", Users: true, Financial: true},
		{Name: "raw/purchases", Query: `SELECT id,user_id,combo_id,charged_txb_minor,gross_price_txb_minor,core_gross_txb_minor,coupon_discount_txb_minor,valid_from,valid_until,status,created_at FROM purchases`, Date: "created_at", User: "user_id", Product: "combo_id", Users: true, Financial: true},
		{Name: "raw/renewals", Query: `SELECT id,user_id,source_purchase_id,term_count,charged_txb_minor,created_at FROM renewal_batches`, Date: "created_at", User: "user_id", Users: true, Financial: true},
		{Name: "raw/purchase_addons", Query: `SELECT a.purchase_id,a.group_id,a.charged_txb_minor,p.user_id,p.combo_id,p.created_at FROM purchase_addons a JOIN purchases p ON p.id=a.purchase_id`, Date: "p.created_at", User: "p.user_id", Product: "p.combo_id", Users: true, Financial: true},
		{Name: "raw/payg_charges", Query: `SELECT c.ledger_id,c.proxy_user_id,c.first_block,c.blocks,c.unit_price_minor,a.user_id,a.node_id,a.group_id,l.created_at FROM xnode_payg_charges c JOIN xnode_payg_accounts a ON a.proxy_user_id=c.proxy_user_id JOIN ledger_entries l ON l.id=c.ledger_id`, Date: "l.created_at", User: "a.user_id", Node: "a.node_id", Product: "a.group_id", Users: true, Financial: true},
		{Name: "raw/offers", Query: `SELECT id,user_id,product_type,product_id,price_minor,reference_minor,exposed_at,expires_at,outcome,outcome_reason,observed_at,purchase_id,model_version,predicted_probability,features_json FROM pricing_offers`, Date: "exposed_at", User: "user_id", Product: "product_id", Users: true, RetentionDays: 60},
		{Name: "features/features", Query: `SELECT id,entity_type,entity_id,day,watermark,coverage_bps,health_state,features_json,snapshot_hash FROM pricing_features`, Date: "day", Users: true, RetentionDays: 90},
		{Name: "forecasts/historical_predictions", Query: `SELECT forecast_id,entity_type,entity_id,metric,generated_at,target_from,target_until,horizon_seconds,predicted_p10,predicted_p50,predicted_p90,predicted_mean,model_version,feature_watermark,feature_snapshot_hash,data_coverage_bps,health_state,calibration_status FROM pricing_forecast_vintages`, Date: "generated_at", Users: true, RetentionDays: 90},
		{Name: "forecasts/prediction_evaluations", Query: `SELECT e.forecast_id,e.actual,e.status,e.observed_until_ns,e.evaluated_at_ns,f.entity_type,f.entity_id FROM pricing_forecast_evaluations e JOIN pricing_forecast_vintages f ON f.forecast_id=e.forecast_id`, Date: "e.evaluated_at_ns", Users: true, RetentionDays: 90},
		{Name: "forecasts/renewal_predictions", Query: `SELECT q.id,q.purchase_id,q.contract_id,q.price_minor,q.policy_version,q.contract_version,q.state,q.outcome_reason,q.generated_at,q.authorized_at,q.settle_after,q.valid_until,q.settled_purchase_id,q.probability,q.contribution,q.health_state,q.model_version,q.trace_json,p.user_id,p.combo_id FROM pricing_renewal_quotes q JOIN purchases p ON p.id=q.purchase_id`, Date: "q.generated_at", User: "p.user_id", Product: "p.combo_id", Users: true, Financial: true},
		{Name: "forecasts/model_versions", Query: `SELECT id,kind,created_at,calibration_status,parameters_json,feature_definition_json,watermark,diagnostics_json,frozen FROM pricing_model_versions`, Shared: true},
		{Name: "pricing/product_settings", Query: `SELECT product_type,product_id,dynamic_enabled,purchase_pricing_mode,renewal_pricing_mode,personal_discounts_enabled,fixed_price_minor,max_increase_bps,max_discount_bps,override_status,last_approved_price_version,version,updated_by,updated_at FROM pricing_product_settings`, Product: "product_id", Shared: true},
		{Name: "pricing/published_prices", Query: `SELECT id,product_type,product_id,price_minor,policy_version,model_version,source,valid_from,valid_until,created_by,created_at,reason FROM pricing_published_prices`, Date: "created_at", Product: "product_id"},
		{Name: "pricing/contracts", Query: `SELECT id,purchase_id,user_id,product_id,mode,contractual_price_minor,price_limit_minor,max_change_bps,policy_version,recorded_at,version,source_purchase_id,addon_terms_json FROM pricing_contracts`, Date: "recorded_at", User: "user_id", Product: "product_id", Users: true, Financial: true},
		{Name: "pricing/personal_decisions", Query: `SELECT version,user_id,product_type,product_id,context_key,policy_version,public_minor,personal_minor,net_minor,status,reason,model_version,probability,calibration,generated_at,valid_until,coupon_json FROM pricing_personal_decisions`, Date: "generated_at", User: "user_id", Product: "product_id", Users: true, Financial: true},
		{Name: "pricing/pricing_decisions", Query: `SELECT id,product_type,product_id,user_id,generated_at,policy_version,model_version,reference_minor,proposed_minor,selected_minor,state,trace_json FROM pricing_decisions`, Date: "generated_at", User: "user_id", Product: "product_id", Users: true, RetentionDays: 60},
		{Name: "pricing/discount_decisions", Query: `SELECT override_id,reference_id,user_id,discount_minor,created_at,status FROM pricing_discount_spend`, Date: "created_at", User: "user_id", Users: true, Financial: true},
		{Name: "pricing/provider_cost_versions", Query: `SELECT node_id,version,cycle_cost_minor,cycle_days,capacity_bytes,effective_at,recorded_at,actor_id,reason,actual_paid_minor FROM pricing_cost_versions`, Date: "recorded_at", Node: "node_id", Financial: true},
		{Name: "validation/calibration", Query: `SELECT day,model_version,kind,bucket,samples,probability_sum,observed_sum,brier_sum,updated_at FROM pricing_calibration_daily`, Date: "day", RetentionDays: 366},
		{Name: "operations/worker_watermark", Query: `SELECT observed_at FROM pricing_worker_watermark`, Shared: true},
		{Name: "pricing/provider_costs", Query: `SELECT node_id,cycle_cost_minor,cycle_days,capacity_bytes,effective_at,updated_by,updated_at,version FROM pricing_cost_metadata`, Date: "effective_at", Node: "node_id", Financial: true},
		{Name: "operations/policy_events", Query: `SELECT id,actor_user_id AS actor_id,action AS event_type,json_extract(detail,'$.value.productType') AS product_type,json_extract(detail,'$.value.productId') AS product_id,target_id AS scope_id,created_at,json_extract(detail,'$.reason') AS reason FROM audit_events WHERE target_type='pricing' AND action='pricing.policy'`, Date: "created_at", Product: "product_id"},
		{Name: "operations/health_incidents", Query: `SELECT id,scope_type,scope_id,state,started_at,ended_at,reason,acknowledged_by,acknowledged_at FROM pricing_operational_incidents`, Date: "started_at", RetentionDays: 90},
		{Name: "operations/admin_overrides", Query: `SELECT id,scope_type,scope_id,command,reason,created_by,starts_at,expires_at,expected_version,previous_json,proposed_json,status,created_at FROM pricing_admin_overrides`, Date: "created_at", Users: true, Financial: true},
		{Name: "validation/model_metrics", Query: `SELECT day,model_version,metric,horizon_seconds,samples,metrics_json FROM pricing_model_validation_daily`, Date: "day", RetentionDays: 366},
	}
}
