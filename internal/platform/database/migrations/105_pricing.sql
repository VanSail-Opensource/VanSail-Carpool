-- Optional per-product policies. No existing catalog row is activated or rewritten.
CREATE TABLE pricing_product_settings (
 product_type TEXT NOT NULL CHECK(product_type IN ('combo','payg','addon')),
 product_id TEXT NOT NULL,
 dynamic_enabled INTEGER NOT NULL DEFAULT 0 CHECK(dynamic_enabled IN (0,1)),
 purchase_pricing_mode TEXT NOT NULL DEFAULT 'FIXED' CHECK(purchase_pricing_mode IN ('FIXED','DYNAMIC')),
 renewal_pricing_mode TEXT NOT NULL DEFAULT 'FIXED' CHECK(renewal_pricing_mode IN ('FIXED','DYNAMIC')),
 personal_discounts_enabled INTEGER NOT NULL DEFAULT 0 CHECK(personal_discounts_enabled IN (0,1)),
 fixed_price_minor INTEGER NOT NULL CHECK(fixed_price_minor>=0 AND fixed_price_minor<=1000000000000),
 max_increase_bps INTEGER NOT NULL DEFAULT 0 CHECK(max_increase_bps BETWEEN 0 AND 10000),
 max_discount_bps INTEGER NOT NULL DEFAULT 0 CHECK(max_discount_bps BETWEEN 0 AND 10000),
 override_status TEXT NOT NULL DEFAULT 'NONE' CHECK(override_status IN ('NONE','FROZEN','MANUAL')),
 last_approved_price_version TEXT NOT NULL DEFAULT '',
 version INTEGER NOT NULL DEFAULT 1,
 updated_by TEXT NOT NULL, updated_at TEXT NOT NULL,
 PRIMARY KEY(product_type,product_id),
 CHECK(dynamic_enabled=1 OR (purchase_pricing_mode='FIXED' AND renewal_pricing_mode='FIXED'))
);
CREATE TABLE pricing_published_prices (
 id TEXT PRIMARY KEY, product_type TEXT NOT NULL, product_id TEXT NOT NULL,
 price_minor INTEGER NOT NULL CHECK(price_minor>=0 AND price_minor<=1000000000000),
 policy_version INTEGER NOT NULL, model_version TEXT NOT NULL,
 source TEXT NOT NULL CHECK(source IN ('FIXED','MANUAL','MODEL')),
 valid_from TEXT NOT NULL, valid_until TEXT NOT NULL, created_by TEXT NOT NULL,
 created_at TEXT NOT NULL, reason TEXT NOT NULL,
 CHECK(valid_until>valid_from)
);
CREATE INDEX pricing_prices_product ON pricing_published_prices(product_type,product_id,created_at);
CREATE TABLE pricing_contracts (
 id TEXT PRIMARY KEY, purchase_id TEXT NOT NULL REFERENCES purchases(id) ON DELETE CASCADE, user_id TEXT NOT NULL REFERENCES users(id),
 product_id TEXT NOT NULL, mode TEXT NOT NULL CHECK(mode IN ('FIXED','DYNAMIC')),
 contractual_price_minor INTEGER NOT NULL CHECK(contractual_price_minor>=0),
 price_limit_minor INTEGER NOT NULL CHECK(price_limit_minor>=0),
 max_change_bps INTEGER NOT NULL CHECK(max_change_bps BETWEEN 0 AND 10000),
 policy_version INTEGER NOT NULL, recorded_at TEXT NOT NULL,
 version INTEGER NOT NULL DEFAULT 1, source_purchase_id TEXT NOT NULL, addon_terms_json TEXT NOT NULL DEFAULT '[]',
 CHECK(price_limit_minor>=contractual_price_minor), UNIQUE(purchase_id,version)
);
CREATE INDEX pricing_contracts_user ON pricing_contracts(user_id,purchase_id);
CREATE TABLE pricing_renewal_quotes (
 id TEXT PRIMARY KEY, purchase_id TEXT NOT NULL REFERENCES purchases(id) ON DELETE CASCADE, contract_id TEXT NOT NULL REFERENCES pricing_contracts(id) ON DELETE CASCADE,
 price_minor INTEGER NOT NULL CHECK(price_minor>=0), policy_version INTEGER NOT NULL,
 contract_version INTEGER NOT NULL, state TEXT NOT NULL CHECK(state IN ('FORECASTED','PREPARED','NOTICE_PENDING','AUTHORIZED','READY_TO_SETTLE','SETTLED','DEFERRED_HEALTH','EXPIRED','CANCELLED')),
 generated_at TEXT NOT NULL, notice_at TEXT, authorized_at TEXT,
 settle_after TEXT NOT NULL, valid_until TEXT NOT NULL, settled_purchase_id TEXT,
 probability DOUBLE PRECISION NOT NULL, contribution DOUBLE PRECISION NOT NULL, health_state TEXT NOT NULL,
 model_version TEXT NOT NULL, trace_json TEXT NOT NULL,
 CHECK(valid_until>settle_after)
);
CREATE INDEX pricing_renewal_purchase ON pricing_renewal_quotes(purchase_id,generated_at);
CREATE TABLE pricing_admin_overrides (
 id TEXT PRIMARY KEY, scope_type TEXT NOT NULL, scope_id TEXT NOT NULL,
 command TEXT NOT NULL, reason TEXT NOT NULL, created_by TEXT NOT NULL,
 starts_at TEXT NOT NULL, expires_at TEXT NOT NULL,
 expected_version INTEGER NOT NULL, previous_json TEXT NOT NULL, proposed_json TEXT NOT NULL,
 status TEXT NOT NULL CHECK(status IN ('ACTIVE','CANCELLED','EXPIRED')),
 created_at TEXT NOT NULL, CHECK(expires_at>starts_at)
);
CREATE INDEX pricing_overrides_scope ON pricing_admin_overrides(scope_type,scope_id,status,expires_at);
CREATE TABLE pricing_operational_incidents (
 id TEXT PRIMARY KEY, scope_type TEXT NOT NULL, scope_id TEXT NOT NULL,
 state TEXT NOT NULL, started_at TEXT NOT NULL, ended_at TEXT,
 reason TEXT NOT NULL, acknowledged_by TEXT, acknowledged_at TEXT
);
CREATE INDEX pricing_incidents_scope ON pricing_operational_incidents(scope_type,scope_id,started_at);
CREATE TABLE pricing_model_versions (
 id TEXT PRIMARY KEY, kind TEXT NOT NULL, created_at TEXT NOT NULL,
 calibration_status TEXT NOT NULL CHECK(calibration_status IN ('VALIDATED','UNVALIDATED','PRIOR_ONLY')),
 parameters_json TEXT NOT NULL, feature_definition_json TEXT NOT NULL,
 watermark TEXT NOT NULL, diagnostics_json TEXT NOT NULL, frozen INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE pricing_features (
 feature_key INTEGER PRIMARY KEY AUTOINCREMENT, id TEXT NOT NULL UNIQUE, entity_type TEXT NOT NULL, entity_id TEXT NOT NULL,
 day TEXT NOT NULL, watermark TEXT NOT NULL, coverage_bps INTEGER NOT NULL CHECK(coverage_bps BETWEEN 0 AND 10000),
 health_state TEXT NOT NULL, features_json TEXT NOT NULL, snapshot_hash TEXT NOT NULL,
 UNIQUE(entity_type,entity_id,day)
);
CREATE INDEX pricing_features_hash ON pricing_features(snapshot_hash,watermark);
CREATE TABLE pricing_forecast_entities (
 entity_key INTEGER PRIMARY KEY AUTOINCREMENT, entity_type TEXT NOT NULL, entity_id TEXT NOT NULL,
 UNIQUE(entity_type,entity_id)
);
CREATE TABLE pricing_forecast_batches (
 batch_key INTEGER PRIMARY KEY AUTOINCREMENT,
 metric TEXT NOT NULL, generated_at TEXT NOT NULL, target_from TEXT NOT NULL,
 target_until TEXT NOT NULL, horizon_seconds INTEGER NOT NULL,
 model_version TEXT NOT NULL, health_state TEXT NOT NULL, calibration_status TEXT NOT NULL,
 CHECK(generated_at<=target_from AND target_until>target_from),
 UNIQUE(metric,generated_at,target_from,target_until,horizon_seconds,model_version,health_state,calibration_status)
);
CREATE INDEX pricing_forecasts_target ON pricing_forecast_batches(target_until,batch_key);
CREATE TABLE pricing_forecast_values (
 forecast_id INTEGER PRIMARY KEY,
 entity_key INTEGER NOT NULL REFERENCES pricing_forecast_entities(entity_key),
 batch_key INTEGER NOT NULL REFERENCES pricing_forecast_batches(batch_key),
 feature_key INTEGER NOT NULL REFERENCES pricing_features(feature_key),
 predicted_p10 DOUBLE PRECISION NOT NULL, predicted_p50 DOUBLE PRECISION NOT NULL, predicted_p90 DOUBLE PRECISION NOT NULL,
 predicted_mean DOUBLE PRECISION NOT NULL,
 data_coverage_bps INTEGER NOT NULL CHECK(data_coverage_bps BETWEEN 0 AND 10000),
 CHECK(predicted_p10<=predicted_p50 AND predicted_p50<=predicted_p90),
 UNIQUE(entity_key,batch_key)
);
CREATE VIEW pricing_forecast_vintages AS
 SELECT v.forecast_id,e.entity_type,e.entity_id,b.metric,b.generated_at,b.target_from,b.target_until,b.horizon_seconds,
 v.predicted_p10,v.predicted_p50,v.predicted_p90,v.predicted_mean,b.model_version,
 f.watermark AS feature_watermark,f.snapshot_hash AS feature_snapshot_hash,v.data_coverage_bps,b.health_state,b.calibration_status
 FROM pricing_forecast_values v JOIN pricing_forecast_entities e ON e.entity_key=v.entity_key
 JOIN pricing_forecast_batches b ON b.batch_key=v.batch_key JOIN pricing_features f ON f.feature_key=v.feature_key;
CREATE TABLE pricing_forecast_actuals (
 forecast_id INTEGER PRIMARY KEY REFERENCES pricing_forecast_values(forecast_id) ON DELETE CASCADE,
 actual DOUBLE PRECISION, status_code SMALLINT NOT NULL CHECK(status_code BETWEEN 0 AND 3),
 observed_until_ns INTEGER NOT NULL, evaluated_at_ns INTEGER NOT NULL,
 CHECK((status_code=0 AND actual IS NOT NULL) OR (status_code<>0 AND actual IS NULL))
);
CREATE VIEW pricing_forecast_evaluations AS SELECT forecast_id,actual,CASE status_code WHEN 0 THEN 'COMPLETE' WHEN 1 THEN 'MISSING' WHEN 2 THEN 'CENSORED' ELSE 'INCOMPLETE' END AS status,observed_until_ns,evaluated_at_ns FROM pricing_forecast_actuals;
CREATE TABLE pricing_model_validation_daily (
 day TEXT NOT NULL, model_version TEXT NOT NULL, metric TEXT NOT NULL,
 horizon_seconds INTEGER NOT NULL, samples INTEGER NOT NULL, metrics_json TEXT NOT NULL, updated_at_ns INTEGER NOT NULL,
 PRIMARY KEY(day,model_version,metric,horizon_seconds)
);
CREATE TABLE pricing_decisions (
 id TEXT PRIMARY KEY, product_type TEXT NOT NULL, product_id TEXT NOT NULL,
 user_id TEXT, generated_at TEXT NOT NULL, policy_version INTEGER NOT NULL,
 model_version TEXT NOT NULL, reference_minor INTEGER NOT NULL, proposed_minor INTEGER NOT NULL,
 selected_minor INTEGER NOT NULL, state TEXT NOT NULL, trace_json TEXT NOT NULL
);
CREATE INDEX pricing_decisions_product ON pricing_decisions(product_type,product_id,generated_at);
CREATE TABLE pricing_cost_metadata (
 node_id TEXT PRIMARY KEY REFERENCES xnode_nodes(id), cycle_cost_minor INTEGER NOT NULL CHECK(cycle_cost_minor>=0),
 cycle_days INTEGER NOT NULL CHECK(cycle_days BETWEEN 1 AND 366),
 capacity_bytes INTEGER NOT NULL CHECK(capacity_bytes>0), effective_at TEXT NOT NULL,
 updated_by TEXT NOT NULL, updated_at TEXT NOT NULL, version INTEGER NOT NULL DEFAULT 1
);
CREATE TABLE pricing_offers (
 id TEXT PRIMARY KEY, user_id TEXT NOT NULL, product_type TEXT NOT NULL, product_id TEXT NOT NULL,
 price_minor INTEGER NOT NULL, reference_minor INTEGER NOT NULL, exposed_at TEXT NOT NULL,
 expires_at TEXT NOT NULL, outcome INTEGER CHECK(outcome IN (0,1)), observed_at TEXT,
 purchase_id TEXT, model_version TEXT NOT NULL, predicted_probability DOUBLE PRECISION NOT NULL,
 features_json TEXT NOT NULL, UNIQUE(user_id,product_type,product_id,price_minor,exposed_at)
);
CREATE INDEX pricing_offers_expiry ON pricing_offers(expires_at,id);
CREATE TABLE pricing_discount_spend (
 override_id TEXT NOT NULL REFERENCES pricing_admin_overrides(id), reference_id TEXT NOT NULL,
 user_id TEXT NOT NULL REFERENCES users(id), discount_minor INTEGER NOT NULL CHECK(discount_minor>=0),
 created_at TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'CONSUMED' CHECK(status IN ('RESERVED','CONSUMED','RELEASED')),
 PRIMARY KEY(override_id,reference_id)
);
CREATE TABLE pricing_worker_watermark (singleton INTEGER PRIMARY KEY CHECK(singleton=1),observed_at TEXT NOT NULL);
INSERT INTO pricing_worker_watermark(singleton,observed_at) VALUES(1,'');
CREATE TRIGGER pricing_forecasts_immutable BEFORE UPDATE ON pricing_forecast_values BEGIN SELECT RAISE(ABORT,'original pricing forecasts are immutable'); END;
CREATE TRIGGER pricing_forecast_batches_immutable BEFORE UPDATE ON pricing_forecast_batches BEGIN SELECT RAISE(ABORT,'original pricing forecast batch metadata is immutable'); END;
CREATE TRIGGER pricing_feature_inputs_immutable BEFORE UPDATE ON pricing_features BEGIN SELECT RAISE(ABORT,'original pricing forecast inputs are immutable'); END;
CREATE TRIGGER pricing_contracts_immutable BEFORE UPDATE ON pricing_contracts BEGIN SELECT RAISE(ABORT,'pricing basis snapshots are immutable'); END;
CREATE TRIGGER pricing_prices_immutable BEFORE UPDATE ON pricing_published_prices BEGIN SELECT RAISE(ABORT,'published pricing facts are immutable'); END;
CREATE TRIGGER pricing_quote_terms_immutable BEFORE UPDATE OF price_minor,contract_id,policy_version,contract_version,generated_at,settle_after,valid_until,model_version,trace_json ON pricing_renewal_quotes BEGIN SELECT RAISE(ABORT,'renewal quote terms are immutable'); END;

-- Versioned provider facts preserve cost inputs and separately declared actual payments.
CREATE TABLE pricing_cost_versions (
 node_id TEXT NOT NULL REFERENCES xnode_nodes(id), version INTEGER NOT NULL,
 cycle_cost_minor INTEGER NOT NULL, cycle_days INTEGER NOT NULL, capacity_bytes INTEGER NOT NULL,
 effective_at TEXT NOT NULL, recorded_at TEXT NOT NULL, actor_id TEXT NOT NULL, reason TEXT NOT NULL,
 actual_paid_minor INTEGER CHECK(actual_paid_minor>=0), PRIMARY KEY(node_id,version)
);
CREATE TABLE pricing_calibration_daily (
 day TEXT NOT NULL, model_version TEXT NOT NULL, kind TEXT NOT NULL,
 bucket INTEGER NOT NULL CHECK(bucket BETWEEN 0 AND 9), samples INTEGER NOT NULL,
 probability_sum DOUBLE PRECISION NOT NULL, observed_sum INTEGER NOT NULL, brier_sum DOUBLE PRECISION NOT NULL,
 updated_at TEXT NOT NULL, PRIMARY KEY(day,model_version,kind,bucket)
);
CREATE TRIGGER pricing_forecast_entities_immutable BEFORE UPDATE ON pricing_forecast_entities BEGIN SELECT RAISE(ABORT,'original forecast entity is immutable'); END;
CREATE TRIGGER pricing_model_parameters_immutable BEFORE UPDATE OF kind,created_at,parameters_json,feature_definition_json,watermark,diagnostics_json,calibration_status ON pricing_model_versions BEGIN SELECT RAISE(ABORT,'original model parameters are immutable'); END;
CREATE TRIGGER pricing_cost_versions_immutable BEFORE UPDATE ON pricing_cost_versions BEGIN SELECT RAISE(ABORT,'original provider cost facts are immutable'); END;
CREATE TRIGGER pricing_offer_inputs_immutable BEFORE UPDATE OF user_id,product_type,product_id,price_minor,reference_minor,exposed_at,expires_at,model_version,predicted_probability,features_json ON pricing_offers BEGIN SELECT RAISE(ABORT,'original price exposures are immutable'); END;

CREATE INDEX pricing_offers_observed ON pricing_offers(exposed_at,model_version);
CREATE INDEX pricing_contract_product ON pricing_contracts(product_id,recorded_at);
CREATE INDEX pricing_values_feature ON pricing_forecast_values(feature_key);

CREATE INDEX pricing_active_commands ON pricing_admin_overrides(command,status,expires_at);
