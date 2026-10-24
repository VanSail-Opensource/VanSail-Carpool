-- +goose Up
-- PAYG financial attribution and future combo entitlement references.

ALTER TABLE squad_product_overrides ADD COLUMN billing_mode TEXT COLLATE "C" NOT NULL DEFAULT 'combo' CHECK(billing_mode IN ('combo','payg'));

ALTER TABLE squad_product_overrides ADD COLUMN payg_price_minor BIGINT NOT NULL DEFAULT 0 CHECK(payg_price_minor>=0 AND (billing_mode<>'payg' OR payg_price_minor>0));

ALTER TABLE purchases ADD COLUMN combo_squad_uuids TEXT COLLATE "C";

CREATE TABLE xnode_payg_accounts (
 proxy_user_id BIGINT PRIMARY KEY REFERENCES xnode_identities(proxy_user_id),
 user_id TEXT COLLATE "C" NOT NULL REFERENCES users(id), group_id TEXT COLLATE "C" NOT NULL,
 node_id TEXT COLLATE "C" NOT NULL REFERENCES xnode_nodes(id),
 period_id TEXT COLLATE "C" NOT NULL UNIQUE REFERENCES xnode_periods(id),
 unit_price_minor BIGINT NOT NULL CHECK(unit_price_minor>0),
 purchased_bytes BIGINT NOT NULL DEFAULT 0 CHECK(purchased_bytes>=0),
 UNIQUE(user_id,group_id,node_id)
);

CREATE TABLE xnode_payg_charges (
 ledger_id TEXT COLLATE "C" PRIMARY KEY REFERENCES ledger_entries(id),
 proxy_user_id BIGINT NOT NULL REFERENCES xnode_payg_accounts(proxy_user_id),
 first_block BIGINT NOT NULL CHECK(first_block>=0),
 blocks BIGINT NOT NULL CHECK(blocks>0),
 unit_price_minor BIGINT NOT NULL CHECK(unit_price_minor>0),
 UNIQUE(proxy_user_id,first_block)
);

CREATE TABLE xnode_payg_minutes (
 user_id TEXT COLLATE "C" NOT NULL REFERENCES users(id), group_id TEXT COLLATE "C" NOT NULL,
 minute TEXT COLLATE "C" NOT NULL, traffic_bytes BIGINT NOT NULL DEFAULT 0 CHECK(traffic_bytes>=0),
 spent_minor BIGINT NOT NULL DEFAULT 0 CHECK(spent_minor>=0),
 PRIMARY KEY(user_id,group_id,minute)
);

CREATE INDEX xnode_payg_minutes_expiry ON xnode_payg_minutes(minute);

CREATE INDEX xnode_payg_node ON xnode_payg_accounts(node_id,user_id);

-- +goose StatementBegin
CREATE FUNCTION "compile_payg_account_insert_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_payg_account_insert" AFTER INSERT ON "xnode_payg_accounts" FOR EACH ROW EXECUTE FUNCTION "compile_payg_account_insert_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_payg_product_delete_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_payg_product_delete" AFTER DELETE ON "squad_product_overrides" FOR EACH ROW EXECUTE FUNCTION "compile_payg_product_delete_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_payg_product_insert_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_payg_product_insert" AFTER INSERT ON "squad_product_overrides" FOR EACH ROW EXECUTE FUNCTION "compile_payg_product_insert_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_payg_product_update_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_payg_product_update" AFTER UPDATE ON "squad_product_overrides" FOR EACH ROW EXECUTE FUNCTION "compile_payg_product_update_fn"();

INSERT INTO schema_migrations(version,applied_at) VALUES('095_payg.sql',vansail_now()) ON CONFLICT(version) DO NOTHING;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
