-- Query indexes only: exact accounting and financial evidence are unchanged.
CREATE INDEX xnode_daily_usage_date_node_idx ON xnode_daily_usage(date,node_id);
CREATE INDEX xnode_daily_usage_user_date_idx ON xnode_daily_usage(proxy_user_id,date,node_id,period_id);
CREATE INDEX xnode_legacy_history_date_node_idx ON xnode_legacy_history(date,node_id);
CREATE INDEX xnode_periods_purchase_idx ON xnode_periods(purchase_id);
CREATE INDEX xnode_service_bindings_expiry_idx ON xnode_service_bindings(valid_until);
