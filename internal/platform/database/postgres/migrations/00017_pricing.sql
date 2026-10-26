-- +goose Up
CREATE TABLE pricing_product_settings (
 product_type TEXT COLLATE "C" NOT NULL CHECK(product_type IN ('combo','payg','addon')),
 product_id TEXT COLLATE "C" NOT NULL,
 dynamic_enabled BIGINT NOT NULL DEFAULT 0 CHECK(dynamic_enabled IN (0,1)),
 purchase_pricing_mode TEXT COLLATE "C" NOT NULL DEFAULT 'FIXED' CHECK(purchase_pricing_mode IN ('FIXED','DYNAMIC')),
 renewal_pricing_mode TEXT COLLATE "C" NOT NULL DEFAULT 'FIXED' CHECK(renewal_pricing_mode IN ('FIXED','DYNAMIC')),
 personal_discounts_enabled BIGINT NOT NULL DEFAULT 0 CHECK(personal_discounts_enabled IN (0,1)),
 fixed_price_minor BIGINT NOT NULL CHECK(fixed_price_minor>=0 AND fixed_price_minor<=1000000000000),
 max_increase_bps BIGINT NOT NULL DEFAULT 0 CHECK(max_increase_bps BETWEEN 0 AND 10000),
 max_discount_bps BIGINT NOT NULL DEFAULT 0 CHECK(max_discount_bps BETWEEN 0 AND 10000),
 override_status TEXT COLLATE "C" NOT NULL DEFAULT 'NONE' CHECK(override_status IN ('NONE','FROZEN','MANUAL')),
 last_approved_price_version TEXT COLLATE "C" NOT NULL DEFAULT '',
 version BIGINT NOT NULL DEFAULT 1,
 updated_by TEXT COLLATE "C" NOT NULL, updated_at TEXT COLLATE "C" NOT NULL,
 PRIMARY KEY(product_type,product_id),
 CHECK(dynamic_enabled=1 OR (purchase_pricing_mode='FIXED' AND renewal_pricing_mode='FIXED'))
);

CREATE TABLE pricing_published_prices (
 id TEXT COLLATE "C" PRIMARY KEY, product_type TEXT COLLATE "C" NOT NULL, product_id TEXT COLLATE "C" NOT NULL,
 price_minor BIGINT NOT NULL CHECK(price_minor>=0 AND price_minor<=1000000000000),
 policy_version BIGINT NOT NULL, model_version TEXT COLLATE "C" NOT NULL,
 source TEXT COLLATE "C" NOT NULL CHECK(source IN ('FIXED','MANUAL','MODEL')),
 valid_from TEXT COLLATE "C" NOT NULL, valid_until TEXT COLLATE "C" NOT NULL, created_by TEXT COLLATE "C" NOT NULL,
 created_at TEXT COLLATE "C" NOT NULL, reason TEXT COLLATE "C" NOT NULL,
 CHECK(valid_until>valid_from)
);

CREATE INDEX pricing_prices_product ON pricing_published_prices(product_type,product_id,created_at);

CREATE TABLE pricing_contracts (
 id TEXT COLLATE "C" PRIMARY KEY, purchase_id TEXT COLLATE "C" NOT NULL REFERENCES purchases(id) ON DELETE CASCADE, user_id TEXT COLLATE "C" NOT NULL REFERENCES users(id),
 product_id TEXT COLLATE "C" NOT NULL, mode TEXT COLLATE "C" NOT NULL CHECK(mode IN ('FIXED','DYNAMIC')),
 contractual_price_minor BIGINT NOT NULL CHECK(contractual_price_minor>=0),
 price_limit_minor BIGINT NOT NULL CHECK(price_limit_minor>=0),
 max_change_bps BIGINT NOT NULL CHECK(max_change_bps BETWEEN 0 AND 10000),
 policy_version BIGINT NOT NULL, recorded_at TEXT COLLATE "C" NOT NULL,
 version BIGINT NOT NULL DEFAULT 1, source_purchase_id TEXT COLLATE "C" NOT NULL, addon_terms_json TEXT COLLATE "C" NOT NULL DEFAULT '[]',
 CHECK(price_limit_minor>=contractual_price_minor), UNIQUE(purchase_id,version)
);

CREATE INDEX pricing_contracts_user ON pricing_contracts(user_id,purchase_id);

CREATE TABLE pricing_renewal_quotes (
 id TEXT COLLATE "C" PRIMARY KEY, purchase_id TEXT COLLATE "C" NOT NULL REFERENCES purchases(id) ON DELETE CASCADE, contract_id TEXT COLLATE "C" NOT NULL REFERENCES pricing_contracts(id) ON DELETE CASCADE,
 price_minor BIGINT NOT NULL CHECK(price_minor>=0), policy_version BIGINT NOT NULL,
 contract_version BIGINT NOT NULL, state TEXT COLLATE "C" NOT NULL CHECK(state IN ('FORECASTED','PREPARED','NOTICE_PENDING','AUTHORIZED','READY_TO_SETTLE','SETTLED','DEFERRED_HEALTH','EXPIRED','CANCELLED')),
 generated_at TEXT COLLATE "C" NOT NULL, notice_at TEXT COLLATE "C", authorized_at TEXT COLLATE "C",
 settle_after TEXT COLLATE "C" NOT NULL, valid_until TEXT COLLATE "C" NOT NULL, settled_purchase_id TEXT COLLATE "C",
 probability DOUBLE PRECISION NOT NULL, contribution DOUBLE PRECISION NOT NULL, health_state TEXT COLLATE "C" NOT NULL,
 model_version TEXT COLLATE "C" NOT NULL, trace_json TEXT COLLATE "C" NOT NULL,
 CHECK(valid_until>settle_after)
);

CREATE INDEX pricing_renewal_purchase ON pricing_renewal_quotes(purchase_id,generated_at);

CREATE TABLE pricing_admin_overrides (
 id TEXT COLLATE "C" PRIMARY KEY, scope_type TEXT COLLATE "C" NOT NULL, scope_id TEXT COLLATE "C" NOT NULL,
 command TEXT COLLATE "C" NOT NULL, reason TEXT COLLATE "C" NOT NULL, created_by TEXT COLLATE "C" NOT NULL,
 starts_at TEXT COLLATE "C" NOT NULL, expires_at TEXT COLLATE "C" NOT NULL,
 expected_version BIGINT NOT NULL, previous_json TEXT COLLATE "C" NOT NULL, proposed_json TEXT COLLATE "C" NOT NULL,
 status TEXT COLLATE "C" NOT NULL CHECK(status IN ('ACTIVE','CANCELLED','EXPIRED')),
 created_at TEXT COLLATE "C" NOT NULL, CHECK(expires_at>starts_at)
);

CREATE INDEX pricing_overrides_scope ON pricing_admin_overrides(scope_type,scope_id,status,expires_at);

CREATE TABLE pricing_operational_incidents (
 id TEXT COLLATE "C" PRIMARY KEY, scope_type TEXT COLLATE "C" NOT NULL, scope_id TEXT COLLATE "C" NOT NULL,
 state TEXT COLLATE "C" NOT NULL, started_at TEXT COLLATE "C" NOT NULL, ended_at TEXT COLLATE "C",
 reason TEXT COLLATE "C" NOT NULL, acknowledged_by TEXT COLLATE "C", acknowledged_at TEXT COLLATE "C"
);

CREATE INDEX pricing_incidents_scope ON pricing_operational_incidents(scope_type,scope_id,started_at);

CREATE TABLE pricing_model_versions (
 id TEXT COLLATE "C" PRIMARY KEY, kind TEXT COLLATE "C" NOT NULL, created_at TEXT COLLATE "C" NOT NULL,
 calibration_status TEXT COLLATE "C" NOT NULL CHECK(calibration_status IN ('VALIDATED','UNVALIDATED','PRIOR_ONLY')),
 parameters_json TEXT COLLATE "C" NOT NULL, feature_definition_json TEXT COLLATE "C" NOT NULL,
 watermark TEXT COLLATE "C" NOT NULL, diagnostics_json TEXT COLLATE "C" NOT NULL, frozen BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE pricing_features (
 feature_key BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY, id TEXT COLLATE "C" NOT NULL UNIQUE, entity_type TEXT COLLATE "C" NOT NULL, entity_id TEXT COLLATE "C" NOT NULL,
 day TEXT COLLATE "C" NOT NULL, watermark TEXT COLLATE "C" NOT NULL, coverage_bps BIGINT NOT NULL CHECK(coverage_bps BETWEEN 0 AND 10000),
 health_state TEXT COLLATE "C" NOT NULL, features_json TEXT COLLATE "C" NOT NULL, snapshot_hash TEXT COLLATE "C" NOT NULL,
 UNIQUE(entity_type,entity_id,day)
);

CREATE INDEX pricing_features_hash ON pricing_features(snapshot_hash,watermark);

CREATE TABLE pricing_forecast_entities (
 entity_key BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY, entity_type TEXT COLLATE "C" NOT NULL, entity_id TEXT COLLATE "C" NOT NULL,
 UNIQUE(entity_type,entity_id)
);

CREATE TABLE pricing_forecast_batches (
 batch_key BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
 metric TEXT COLLATE "C" NOT NULL, generated_at TEXT COLLATE "C" NOT NULL, target_from TEXT COLLATE "C" NOT NULL,
 target_until TEXT COLLATE "C" NOT NULL, horizon_seconds BIGINT NOT NULL,
 model_version TEXT COLLATE "C" NOT NULL, health_state TEXT COLLATE "C" NOT NULL, calibration_status TEXT COLLATE "C" NOT NULL,
 CHECK(generated_at<=target_from AND target_until>target_from),
 UNIQUE(metric,generated_at,target_from,target_until,horizon_seconds,model_version,health_state,calibration_status)
);

CREATE INDEX pricing_forecasts_target ON pricing_forecast_batches(target_until,batch_key);

CREATE TABLE pricing_forecast_values (
 forecast_id BIGINT PRIMARY KEY,
 entity_key BIGINT NOT NULL REFERENCES pricing_forecast_entities(entity_key),
 batch_key BIGINT NOT NULL REFERENCES pricing_forecast_batches(batch_key),
 feature_key BIGINT NOT NULL REFERENCES pricing_features(feature_key),
 predicted_p10 DOUBLE PRECISION NOT NULL, predicted_p50 DOUBLE PRECISION NOT NULL, predicted_p90 DOUBLE PRECISION NOT NULL,
 predicted_mean DOUBLE PRECISION NOT NULL,
 data_coverage_bps BIGINT NOT NULL CHECK(data_coverage_bps BETWEEN 0 AND 10000),
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
 forecast_id BIGINT PRIMARY KEY REFERENCES pricing_forecast_values(forecast_id) ON DELETE CASCADE,
 actual DOUBLE PRECISION, status_code SMALLINT NOT NULL CHECK(status_code BETWEEN 0 AND 3),
 observed_until_ns BIGINT NOT NULL, evaluated_at_ns BIGINT NOT NULL,
 CHECK((status_code=0 AND actual IS NOT NULL) OR (status_code<>0 AND actual IS NULL))
);

CREATE VIEW pricing_forecast_evaluations AS SELECT forecast_id,actual,CASE status_code WHEN 0 THEN 'COMPLETE' WHEN 1 THEN 'MISSING' WHEN 2 THEN 'CENSORED' ELSE 'INCOMPLETE' END AS status,observed_until_ns,evaluated_at_ns FROM pricing_forecast_actuals;

CREATE TABLE pricing_model_validation_daily (
 day TEXT COLLATE "C" NOT NULL, model_version TEXT COLLATE "C" NOT NULL, metric TEXT COLLATE "C" NOT NULL,
 horizon_seconds BIGINT NOT NULL, samples BIGINT NOT NULL, metrics_json TEXT COLLATE "C" NOT NULL, updated_at_ns BIGINT NOT NULL,
 PRIMARY KEY(day,model_version,metric,horizon_seconds)
);

CREATE TABLE pricing_decisions (
 id TEXT COLLATE "C" PRIMARY KEY, product_type TEXT COLLATE "C" NOT NULL, product_id TEXT COLLATE "C" NOT NULL,
 user_id TEXT COLLATE "C", generated_at TEXT COLLATE "C" NOT NULL, policy_version BIGINT NOT NULL,
 model_version TEXT COLLATE "C" NOT NULL, reference_minor BIGINT NOT NULL, proposed_minor BIGINT NOT NULL,
 selected_minor BIGINT NOT NULL, state TEXT COLLATE "C" NOT NULL, trace_json TEXT COLLATE "C" NOT NULL
);

CREATE INDEX pricing_decisions_product ON pricing_decisions(product_type,product_id,generated_at);

CREATE TABLE pricing_cost_metadata (
 node_id TEXT COLLATE "C" PRIMARY KEY REFERENCES xnode_nodes(id), cycle_cost_minor BIGINT NOT NULL CHECK(cycle_cost_minor>=0),
 cycle_days BIGINT NOT NULL CHECK(cycle_days BETWEEN 1 AND 366),
 capacity_bytes BIGINT NOT NULL CHECK(capacity_bytes>0), effective_at TEXT COLLATE "C" NOT NULL,
 updated_by TEXT COLLATE "C" NOT NULL, updated_at TEXT COLLATE "C" NOT NULL, version BIGINT NOT NULL DEFAULT 1
);

CREATE TABLE pricing_offers (
 id TEXT COLLATE "C" PRIMARY KEY, user_id TEXT COLLATE "C" NOT NULL, product_type TEXT COLLATE "C" NOT NULL, product_id TEXT COLLATE "C" NOT NULL,
 price_minor BIGINT NOT NULL, reference_minor BIGINT NOT NULL, exposed_at TEXT COLLATE "C" NOT NULL,
 expires_at TEXT COLLATE "C" NOT NULL, outcome BIGINT CHECK(outcome IN (0,1)), observed_at TEXT COLLATE "C",
 purchase_id TEXT COLLATE "C", model_version TEXT COLLATE "C" NOT NULL, predicted_probability DOUBLE PRECISION NOT NULL,
 features_json TEXT COLLATE "C" NOT NULL, UNIQUE(user_id,product_type,product_id,price_minor,exposed_at)
);

CREATE INDEX pricing_offers_expiry ON pricing_offers(expires_at,id);

CREATE TABLE pricing_discount_spend (
 override_id TEXT COLLATE "C" NOT NULL REFERENCES pricing_admin_overrides(id), reference_id TEXT COLLATE "C" NOT NULL,
 user_id TEXT COLLATE "C" NOT NULL REFERENCES users(id), discount_minor BIGINT NOT NULL CHECK(discount_minor>=0),
 created_at TEXT COLLATE "C" NOT NULL, status TEXT COLLATE "C" NOT NULL DEFAULT 'CONSUMED' CHECK(status IN ('RESERVED','CONSUMED','RELEASED')),
 PRIMARY KEY(override_id,reference_id)
);

CREATE TABLE pricing_worker_watermark (singleton BIGINT PRIMARY KEY CHECK(singleton=1),observed_at TEXT COLLATE "C" NOT NULL);

INSERT INTO pricing_worker_watermark(singleton,observed_at) VALUES(1,'');

-- +goose StatementBegin
CREATE FUNCTION "pricing_forecasts_immutable_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
RAISE EXCEPTION USING MESSAGE='original pricing forecasts are immutable', ERRCODE='23514';
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "pricing_forecasts_immutable" BEFORE UPDATE ON "pricing_forecast_values" FOR EACH ROW EXECUTE FUNCTION "pricing_forecasts_immutable_fn"();

-- +goose StatementBegin
CREATE FUNCTION "pricing_forecast_batches_immutable_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
RAISE EXCEPTION USING MESSAGE='original pricing forecast batch metadata is immutable', ERRCODE='23514';
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "pricing_forecast_batches_immutable" BEFORE UPDATE ON "pricing_forecast_batches" FOR EACH ROW EXECUTE FUNCTION "pricing_forecast_batches_immutable_fn"();

-- +goose StatementBegin
CREATE FUNCTION "pricing_feature_inputs_immutable_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
RAISE EXCEPTION USING MESSAGE='original pricing forecast inputs are immutable', ERRCODE='23514';
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "pricing_feature_inputs_immutable" BEFORE UPDATE ON "pricing_features" FOR EACH ROW EXECUTE FUNCTION "pricing_feature_inputs_immutable_fn"();

-- +goose StatementBegin
CREATE FUNCTION "pricing_contracts_immutable_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
RAISE EXCEPTION USING MESSAGE='pricing basis snapshots are immutable', ERRCODE='23514';
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "pricing_contracts_immutable" BEFORE UPDATE ON "pricing_contracts" FOR EACH ROW EXECUTE FUNCTION "pricing_contracts_immutable_fn"();

-- +goose StatementBegin
CREATE FUNCTION "pricing_prices_immutable_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
RAISE EXCEPTION USING MESSAGE='published pricing facts are immutable', ERRCODE='23514';
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "pricing_prices_immutable" BEFORE UPDATE ON "pricing_published_prices" FOR EACH ROW EXECUTE FUNCTION "pricing_prices_immutable_fn"();

-- +goose StatementBegin
CREATE FUNCTION "pricing_quote_terms_immutable_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
RAISE EXCEPTION USING MESSAGE='renewal quote terms are immutable', ERRCODE='23514';
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "pricing_quote_terms_immutable" BEFORE UPDATE OF price_minor,contract_id,policy_version,contract_version,generated_at,settle_after,valid_until,model_version,trace_json ON "pricing_renewal_quotes" FOR EACH ROW EXECUTE FUNCTION "pricing_quote_terms_immutable_fn"();

CREATE TABLE pricing_cost_versions (
 node_id TEXT COLLATE "C" NOT NULL REFERENCES xnode_nodes(id), version BIGINT NOT NULL,
 cycle_cost_minor BIGINT NOT NULL, cycle_days BIGINT NOT NULL, capacity_bytes BIGINT NOT NULL,
 effective_at TEXT COLLATE "C" NOT NULL, recorded_at TEXT COLLATE "C" NOT NULL, actor_id TEXT COLLATE "C" NOT NULL, reason TEXT COLLATE "C" NOT NULL,
 actual_paid_minor BIGINT CHECK(actual_paid_minor>=0), PRIMARY KEY(node_id,version)
);

CREATE TABLE pricing_calibration_daily (
 day TEXT COLLATE "C" NOT NULL, model_version TEXT COLLATE "C" NOT NULL, kind TEXT COLLATE "C" NOT NULL,
 bucket BIGINT NOT NULL CHECK(bucket BETWEEN 0 AND 9), samples BIGINT NOT NULL,
 probability_sum DOUBLE PRECISION NOT NULL, observed_sum BIGINT NOT NULL, brier_sum DOUBLE PRECISION NOT NULL,
 updated_at TEXT COLLATE "C" NOT NULL, PRIMARY KEY(day,model_version,kind,bucket)
);

-- +goose StatementBegin
CREATE FUNCTION "pricing_forecast_entities_immutable_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
RAISE EXCEPTION USING MESSAGE='original forecast entity is immutable', ERRCODE='23514';
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "pricing_forecast_entities_immutable" BEFORE UPDATE ON "pricing_forecast_entities" FOR EACH ROW EXECUTE FUNCTION "pricing_forecast_entities_immutable_fn"();

-- +goose StatementBegin
CREATE FUNCTION "pricing_model_parameters_immutable_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
RAISE EXCEPTION USING MESSAGE='original model parameters are immutable', ERRCODE='23514';
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "pricing_model_parameters_immutable" BEFORE UPDATE OF kind,created_at,parameters_json,feature_definition_json,watermark,diagnostics_json,calibration_status ON "pricing_model_versions" FOR EACH ROW EXECUTE FUNCTION "pricing_model_parameters_immutable_fn"();

-- +goose StatementBegin
CREATE FUNCTION "pricing_cost_versions_immutable_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
RAISE EXCEPTION USING MESSAGE='original provider cost facts are immutable', ERRCODE='23514';
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "pricing_cost_versions_immutable" BEFORE UPDATE ON "pricing_cost_versions" FOR EACH ROW EXECUTE FUNCTION "pricing_cost_versions_immutable_fn"();

-- +goose StatementBegin
CREATE FUNCTION "pricing_offer_inputs_immutable_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
RAISE EXCEPTION USING MESSAGE='original price exposures are immutable', ERRCODE='23514';
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "pricing_offer_inputs_immutable" BEFORE UPDATE OF user_id,product_type,product_id,price_minor,reference_minor,exposed_at,expires_at,model_version,predicted_probability,features_json ON "pricing_offers" FOR EACH ROW EXECUTE FUNCTION "pricing_offer_inputs_immutable_fn"();

CREATE INDEX pricing_offers_observed ON pricing_offers(exposed_at,model_version);

CREATE INDEX pricing_contract_product ON pricing_contracts(product_id,recorded_at);

CREATE INDEX pricing_values_feature ON pricing_forecast_values(feature_key);

INSERT INTO schema_migrations(version,applied_at) VALUES('105_pricing.sql',vansail_now()) ON CONFLICT(version) DO NOTHING;
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
