-- +goose Up
-- Final SQLite schema; exact timestamp text retained by deployment-owner decision.

CREATE TABLE activity_daily_rollups (
  local_date TEXT PRIMARY KEY,
  checkin_count BIGINT NOT NULL DEFAULT 0 CHECK (checkin_count >= 0),
  checkin_reward_txb_minor BIGINT NOT NULL DEFAULT 0 CHECK (checkin_reward_txb_minor >= 0),
  group_message_count BIGINT NOT NULL DEFAULT 0 CHECK (group_message_count >= 0),
  group_message_reward_txb_minor BIGINT NOT NULL DEFAULT 0 CHECK (group_message_reward_txb_minor >= 0),
  updated_at TEXT NOT NULL
);

CREATE TABLE activity_draw_delivery (
  delivery_key TEXT PRIMARY KEY,
  chat_id BIGINT NOT NULL,
  message_id BIGINT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE activity_games (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  icon TEXT NOT NULL DEFAULT '',
  enabled BIGINT NOT NULL DEFAULT 0,
  win_chance_bps BIGINT NOT NULL CHECK (win_chance_bps BETWEEN 0 AND 10000),
  minimum_stake_minor BIGINT NOT NULL CHECK (minimum_stake_minor > 0),
  maximum_stake_minor BIGINT NOT NULL CHECK (maximum_stake_minor >= minimum_stake_minor),
  return_multiplier_bps BIGINT NOT NULL CHECK (return_multiplier_bps BETWEEN 10000 AND 1000000),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
, description TEXT NOT NULL DEFAULT '');

CREATE TABLE activity_group_message_raw_events (
  chat_id BIGINT NOT NULL,
  message_id BIGINT NOT NULL,
  local_date TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY (chat_id,message_id)
);

CREATE TABLE activity_lucky_draws (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  kind TEXT NOT NULL CHECK (kind IN ('instant','raffle')),
  status TEXT NOT NULL CHECK (status IN ('draft','publishing','open','settling','completed','cancelled')),
  fee_minor BIGINT NOT NULL CHECK (fee_minor > 0),
  expected_participation BIGINT CHECK (expected_participation IS NULL OR expected_participation > 0),
  threshold BIGINT CHECK (threshold IS NULL OR threshold > 0),
  keyword TEXT NOT NULL DEFAULT '',
  command TEXT NOT NULL DEFAULT '',
  group_chat_id BIGINT,
  announcement_message_id BIGINT,
  revision BIGINT NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  CHECK ((kind='instant' AND expected_participation IS NOT NULL AND threshold IS NULL)
      OR (kind='raffle' AND threshold IS NOT NULL AND expected_participation IS NULL))
);

CREATE TABLE combos (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  price_txb_minor BIGINT NOT NULL CHECK (price_txb_minor >= 0),
  validity_days BIGINT NOT NULL CHECK (validity_days BETWEEN 1 AND 3650),
  traffic_limit_bytes BIGINT NOT NULL CHECK (traffic_limit_bytes > 0),
  active BIGINT NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
, rollover_min_remaining_bps BIGINT NOT NULL DEFAULT 0
  CHECK (rollover_min_remaining_bps BETWEEN 0 AND 10000), included_squad_uuids TEXT NOT NULL DEFAULT '[]'
  CHECK (json_valid(included_squad_uuids) AND json_type(included_squad_uuids)='array'), reset_strategy TEXT NOT NULL DEFAULT 'MONTH_ROLLING'
  CHECK (reset_strategy IN ('DAY','WEEK','MONTH_ROLLING')), ip_lookup_quota BIGINT CHECK(ip_lookup_quota IS NULL OR ip_lookup_quota BETWEEN 0 AND 1000000), whitelist_ip_quota BIGINT NOT NULL DEFAULT 5
 CHECK(whitelist_ip_quota BETWEEN 0 AND 1000));

CREATE TABLE coupon_definitions (
  id TEXT PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN ('purchase_recurring','purchase_once','balance_add','balance_multiply')),
  discount_mode TEXT NOT NULL DEFAULT '' CHECK (discount_mode IN ('','fixed','percent')),
  value_minor_or_bps BIGINT NOT NULL CHECK (value_minor_or_bps > 0),
  percent_cap_minor BIGINT CHECK (percent_cap_minor IS NULL OR percent_cap_minor > 0),
  eligible_combo_ids TEXT NOT NULL DEFAULT '[]',
  eligible_squad_ids TEXT NOT NULL DEFAULT '[]',
  expires_at TEXT,
  global_use_limit BIGINT CHECK (global_use_limit IS NULL OR global_use_limit > 0),
  per_user_use_limit BIGINT CHECK (per_user_use_limit IS NULL OR per_user_use_limit > 0),
  active BIGINT NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
, admin_visible BIGINT NOT NULL DEFAULT 1 CHECK (admin_visible IN (0,1)));

CREATE TABLE host_connectivity_attempts (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL,
    host_uuid TEXT,
    config_hash TEXT NOT NULL CHECK (length(config_hash)=64),
    proxy_user_id BIGINT NOT NULL CHECK (proxy_user_id>0),
    trigger TEXT NOT NULL CHECK (trigger IN ('manual','scheduled')),
    started_at TEXT NOT NULL CHECK (length(started_at)=30),
    finished_at TEXT CHECK (finished_at IS NULL OR (length(finished_at)=30 AND finished_at>=started_at)),
    status TEXT NOT NULL CHECK (status IN ('running','connected','failed','unsupported','error','interrupted')),
    latency_ms DOUBLE PRECISION CHECK (latency_ms IS NULL OR latency_ms>=0),
    http_status BIGINT CHECK (http_status IS NULL OR http_status BETWEEN 100 AND 599),
    error_code TEXT NOT NULL DEFAULT '',
    CHECK ((status='running' AND finished_at IS NULL AND latency_ms IS NULL AND http_status IS NULL AND error_code='')
        OR (status<>'running' AND finished_at IS NOT NULL))
);

CREATE TABLE ip_lookup_live_snapshots (
    id TEXT PRIMARY KEY,
    ip TEXT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('succeeded','partial')),
    details_json TEXT NOT NULL,
    completed_at TEXT NOT NULL,
    run_key TEXT UNIQUE
);

CREATE TABLE ip_lookup_reports (
    id TEXT PRIMARY KEY,
    ip TEXT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('processing','succeeded','partial','failed')),
    report_json TEXT NOT NULL,
    created_at TEXT NOT NULL,
    completed_at TEXT
, live_state_json TEXT NOT NULL DEFAULT '{}', heartbeat_at TEXT, lease_owner TEXT, lease_expires_at TEXT, cache_key TEXT NOT NULL DEFAULT '');

CREATE TABLE member_identity_aliases (
 old_user_id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL,
 old_proxy_id BIGINT UNIQUE,
 proxy_id BIGINT UNIQUE,
 CHECK ((old_proxy_id IS NULL)=(proxy_id IS NULL))
);

CREATE TABLE node_compensation_config (
  id BIGINT PRIMARY KEY CHECK (id = 1),
  enabled BIGINT NOT NULL DEFAULT 0 CHECK (enabled IN (0,1)),
  threshold_minutes BIGINT CHECK (threshold_minutes BETWEEN 1 AND 5256000),
  multiplier_bps BIGINT CHECK (multiplier_bps BETWEEN 100 AND 1000000),
  revision BIGINT NOT NULL DEFAULT 0 CHECK (revision >= 0),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  CHECK (enabled = 0 OR (threshold_minutes IS NOT NULL AND multiplier_bps IS NOT NULL))
);

CREATE TABLE "outbox_jobs" (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  payload TEXT NOT NULL CHECK (json_valid(payload)),
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','processing','done','failed')),
  attempts BIGINT NOT NULL DEFAULT 0,
  available_at TEXT NOT NULL,
  last_error TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE payment_callback_tombstones (
  provider TEXT NOT NULL,
  dedupe_key TEXT NOT NULL,
  order_id TEXT NOT NULL,
  payload_hash TEXT NOT NULL,
  received_at TEXT NOT NULL,
  processed_at TEXT NOT NULL,
  PRIMARY KEY(provider,dedupe_key)
);

CREATE TABLE "payment_profiles" (
  id TEXT PRIMARY KEY,
  provider TEXT NOT NULL CHECK (provider IN ('ezpay','bepusdt')),
  provider_name TEXT NOT NULL,
  enabled_channels TEXT NOT NULL DEFAULT '',
  endpoint TEXT NOT NULL,
  merchant_id TEXT NOT NULL DEFAULT '',
  credential_ciphertext TEXT NOT NULL DEFAULT '',
  acknowledgement TEXT NOT NULL DEFAULT '',
  enabled BIGINT NOT NULL DEFAULT 0 CHECK (enabled IN (0,1)),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE payment_status_rollups (
  local_date TEXT NOT NULL,
  provider TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('paid','expired','cancelled','failed','refunded')),
  order_count BIGINT NOT NULL DEFAULT 0 CHECK (order_count >= 0),
  txb_minor BIGINT NOT NULL DEFAULT 0 CHECK (txb_minor >= 0),
  updated_at TEXT NOT NULL,
  PRIMARY KEY(local_date,provider,status)
);

CREATE TABLE purchase_history_tombstones (
  purchase_id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  combo_id TEXT NOT NULL,
  status TEXT NOT NULL,
  charged_txb_minor BIGINT NOT NULL CHECK (charged_txb_minor >= 0),
  gross_txb_minor BIGINT NOT NULL CHECK (gross_txb_minor >= 0),
  core_gross_txb_minor BIGINT NOT NULL CHECK (core_gross_txb_minor >= 0),
  coupon_discount_txb_minor BIGINT NOT NULL CHECK (coupon_discount_txb_minor >= 0),
  addon_count BIGINT NOT NULL DEFAULT 0 CHECK (addon_count >= 0),
  traffic_limit_bytes BIGINT NOT NULL CHECK (traffic_limit_bytes > 0),
  reset_strategy TEXT NOT NULL CHECK (reset_strategy IN ('DAY','WEEK','MONTH_ROLLING')),
  valid_from TEXT NOT NULL,
  valid_until TEXT NOT NULL,
  renewal_batch_id TEXT NOT NULL DEFAULT '',
  renewal_index BIGINT,
  auto_renew_source_purchase_id TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  removed_at TEXT NOT NULL
);

CREATE TABLE questionnaires (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  form_url TEXT NOT NULL,
  reward_txb_minor BIGINT NOT NULL CHECK (reward_txb_minor >= 0),
  status TEXT NOT NULL CHECK (status IN ('draft','active','closed','settling','settled')),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
, closes_at TEXT);

CREATE TABLE rollover_member_daily_rollups (
  local_date TEXT NOT NULL,
  user_id TEXT NOT NULL,
  settlement_count BIGINT NOT NULL DEFAULT 0 CHECK (settlement_count >= 0),
  credited_txb_minor BIGINT NOT NULL DEFAULT 0 CHECK (credited_txb_minor >= 0),
  allocated_traffic_bytes BIGINT NOT NULL DEFAULT 0 CHECK (allocated_traffic_bytes >= 0),
  used_traffic_bytes BIGINT NOT NULL DEFAULT 0 CHECK (used_traffic_bytes >= 0),
  updated_at TEXT NOT NULL,
  PRIMARY KEY(local_date,user_id)
);

CREATE TABLE schema_migrations (version TEXT PRIMARY KEY, applied_at TEXT NOT NULL);

CREATE TABLE squad_product_overrides (
  group_id TEXT PRIMARY KEY,
  description TEXT NOT NULL DEFAULT '',
  price_txb_minor BIGINT NOT NULL DEFAULT 0 CHECK (price_txb_minor >= 0),
  visible BIGINT NOT NULL DEFAULT 0 CHECK (visible IN (0,1)),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
, stock_limit BIGINT
  CHECK (stock_limit IS NULL OR stock_limit >= 0), profile_json TEXT
  CHECK (profile_json IS NULL OR json_valid(profile_json)), activation_required BIGINT NOT NULL DEFAULT 0
  CHECK (activation_required IN (0,1)), activation_code_hash TEXT);

CREATE TABLE statistics_snapshots (
  partition TEXT PRIMARY KEY CHECK (length(partition) BETWEEN 1 AND 80),
  payload_json TEXT NOT NULL
    CHECK (json_valid(payload_json) AND json_type(payload_json)='object'),
  generated_at TEXT NOT NULL
);

CREATE TABLE telegram_boost_receipts (
    chat_id BIGINT NOT NULL,
    boost_id TEXT NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (chat_id, boost_id)
);

CREATE TABLE telegram_pm_updates (
    update_id BIGINT PRIMARY KEY,
    created_at TEXT NOT NULL
);

CREATE TABLE usdt_quotes (
 order_id TEXT PRIMARY KEY, network TEXT NOT NULL, token TEXT NOT NULL,
 recipient TEXT NOT NULL, amount TEXT NOT NULL, atomic_amount TEXT NOT NULL,
 expires_at TEXT NOT NULL,
 UNIQUE(network,token,recipient,atomic_amount)
);

CREATE TABLE "usdt_scan_cursors" (
 network TEXT NOT NULL, token TEXT NOT NULL, value TEXT NOT NULL,
 hash TEXT NOT NULL, updated_at TEXT NOT NULL, PRIMARY KEY(network,token)
);

CREATE TABLE "usdt_transfers" (
 id TEXT PRIMARY KEY, network TEXT NOT NULL, token TEXT NOT NULL, recipient TEXT NOT NULL,
 atomic_amount TEXT NOT NULL, amount TEXT NOT NULL, tx_hash TEXT NOT NULL, event_id TEXT NOT NULL,
 occurred_at TEXT NOT NULL, order_id TEXT NOT NULL DEFAULT '',
 status TEXT NOT NULL CHECK(status IN ('ready','review','credited','dismissed')),
 reason TEXT NOT NULL DEFAULT '', received_at TEXT NOT NULL,
 dismissed_by TEXT, dismissed_reason TEXT, dismissed_at TEXT,
 UNIQUE(network,tx_hash,event_id),
 CHECK((status='dismissed')=(dismissed_by IS NOT NULL AND dismissed_reason IS NOT NULL AND dismissed_at IS NOT NULL))
);

CREATE TABLE "users" (
  id TEXT PRIMARY KEY,
  telegram_id BIGINT NOT NULL UNIQUE,
  telegram_first_name TEXT NOT NULL DEFAULT '',
  telegram_last_name TEXT NOT NULL DEFAULT '',
  telegram_username TEXT NOT NULL DEFAULT '',
  
  role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'admin')),
  onboarding_state TEXT NOT NULL DEFAULT 'intro' CHECK (onboarding_state IN ('intro','membership','username','agreement','complete')),
  group_joined BIGINT NOT NULL DEFAULT 0,
  channel_joined BIGINT NOT NULL DEFAULT 0,
  policy_accepted_at TEXT,
  proxy_user_id TEXT UNIQUE,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
, recovery_reason TEXT NOT NULL DEFAULT '', accepted_agreement_revision BIGINT NOT NULL DEFAULT 0, new_user BIGINT NOT NULL DEFAULT 0 CHECK (new_user IN (0,1)), inviter_id BIGINT REFERENCES users(telegram_id), notification_locale TEXT NOT NULL DEFAULT 'en' CHECK (notification_locale IN ('en','zh-CN')), auto_traffic_reset_enabled BIGINT NOT NULL DEFAULT 1
  CHECK (auto_traffic_reset_enabled IN (0,1)), display_currency TEXT NOT NULL DEFAULT 'TXB'
  CHECK (display_currency IN ('TXB','CNY','USD')), pm_blocked BIGINT NOT NULL DEFAULT 0 CHECK (pm_blocked IN (0,1)), pm_muted BIGINT NOT NULL DEFAULT 0 CHECK (pm_muted IN (0,1)));

CREATE TABLE xnode_compile_revision (singleton BIGINT PRIMARY KEY CHECK(singleton=1), revision BIGINT NOT NULL);

CREATE TABLE xnode_configs (
 id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, document_cipher TEXT NOT NULL,
 revision BIGINT NOT NULL DEFAULT 1, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);

CREATE TABLE xnode_firewall_bans(
 generation TEXT PRIMARY KEY,prefix TEXT NOT NULL,ip TEXT NOT NULL,rule TEXT NOT NULL,detected_count TEXT NOT NULL,detected_at TEXT NOT NULL,
 until_at TEXT NOT NULL,refresh_seconds BIGINT NOT NULL,issued_version BIGINT NOT NULL,released_version BIGINT NOT NULL DEFAULT 0,
 released_at TEXT,participants BYTEA NOT NULL,expired BYTEA NOT NULL
);

CREATE TABLE xnode_firewall_clock(singleton BIGINT PRIMARY KEY CHECK(singleton=1),version BIGINT NOT NULL,source_version BIGINT NOT NULL,inventory_version BIGINT NOT NULL,active_bans BIGINT NOT NULL DEFAULT 0);

CREATE TABLE xnode_firewall_members(
 ordinal BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY, node_id TEXT NOT NULL UNIQUE,
 active BIGINT NOT NULL DEFAULT 1, applied BIGINT NOT NULL DEFAULT 0, source_applied BIGINT NOT NULL DEFAULT 0,
 force_sync BIGINT NOT NULL DEFAULT 1,connection_epoch BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE xnode_firewall_sources(
 id TEXT NOT NULL,generation TEXT NOT NULL,until_at TEXT NOT NULL,refresh_seconds BIGINT NOT NULL,
 issued_version BIGINT NOT NULL,released_version BIGINT NOT NULL DEFAULT 0,released_at TEXT,participants BYTEA NOT NULL,expired BYTEA NOT NULL,PRIMARY KEY(id,generation)
);

CREATE TABLE xnode_groups (
 id TEXT PRIMARY KEY, name TEXT NOT NULL, enabled BIGINT NOT NULL DEFAULT 1 CHECK(enabled IN(0,1)),
 created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);

CREATE TABLE "xnode_node_rates" (
 agent_id TEXT NOT NULL, from_sequence BIGINT NOT NULL,
 multiplier_fp BIGINT NOT NULL CHECK(multiplier_fp>=0), PRIMARY KEY(agent_id,from_sequence)
);

CREATE TABLE xnode_snippets (
 id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, kind TEXT NOT NULL CHECK(kind IN ('outbound','routing')),
 document_cipher TEXT NOT NULL, revision BIGINT NOT NULL DEFAULT 1,
 created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);

CREATE TABLE abuse_incident_facts (
  incident_id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  incident_bucket_at TEXT NOT NULL,
  selected_action TEXT NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE(user_id,incident_bucket_at)
);

CREATE TABLE abuse_records (
  id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, incident_bucket_at TEXT NOT NULL,
  measured_qps BIGINT NOT NULL CHECK(measured_qps >= 0), qps_limit BIGINT NOT NULL CHECK(qps_limit >= 1), selected_action TEXT NOT NULL,
  expires_at TEXT, deleted_at TEXT, created_at TEXT NOT NULL, punishment_completed_at TEXT, UNIQUE(user_id,incident_bucket_at)
);

CREATE TABLE activity_bets (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  game_id TEXT NOT NULL REFERENCES activity_games(id),
  stake_minor BIGINT NOT NULL CHECK (stake_minor > 0),
  won BIGINT NOT NULL,
  payout_minor BIGINT NOT NULL CHECK (payout_minor >= 0),
  balance_after_minor BIGINT NOT NULL CHECK (balance_after_minor >= 0),
  configuration_snapshot TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE(user_id, idempotency_key)
);

CREATE TABLE activity_daily_checkins (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  local_date TEXT NOT NULL,
  timezone TEXT NOT NULL,
  reward_minor BIGINT NOT NULL CHECK (reward_minor >= 0),
  balance_after_minor BIGINT NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE(user_id, local_date)
);

CREATE TABLE "activity_draw_results" (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  draw_id TEXT NOT NULL,
  prize_id TEXT NOT NULL,
  prize_name TEXT NOT NULL,
  fee_minor BIGINT NOT NULL CHECK (fee_minor >= 0),
  reward_kind TEXT NOT NULL,
  reward_payload TEXT NOT NULL,
  balance_after_minor BIGINT NOT NULL CHECK (balance_after_minor >= 0),
  configuration_snapshot TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE(user_id, idempotency_key)
);

CREATE TABLE activity_draw_revisions (
  id TEXT PRIMARY KEY,
  draw_id TEXT NOT NULL REFERENCES activity_lucky_draws(id) ON DELETE CASCADE,
  revision BIGINT NOT NULL,
  snapshot_json TEXT NOT NULL CHECK (json_valid(snapshot_json)),
  created_at TEXT NOT NULL,
  UNIQUE(draw_id,revision)
);

CREATE TABLE activity_group_message_events (
  chat_id BIGINT NOT NULL,
  message_id BIGINT NOT NULL,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  local_date TEXT NOT NULL,
  counted BIGINT NOT NULL DEFAULT 0 CHECK (counted IN (0,1)),
  created_at TEXT NOT NULL,
  PRIMARY KEY (chat_id, message_id)
);

CREATE TABLE activity_group_message_windows (
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  local_date TEXT NOT NULL,
  message_count BIGINT NOT NULL DEFAULT 0 CHECK (message_count >= 0),
  rewarded_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (user_id, local_date)
);

CREATE TABLE activity_lucky_prizes (
  id TEXT PRIMARY KEY,
  draw_id TEXT NOT NULL REFERENCES activity_lucky_draws(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  position BIGINT NOT NULL CHECK (position >= 0),
  probability_bps BIGINT CHECK (probability_bps IS NULL OR probability_bps BETWEEN 1 AND 10000),
  stock BIGINT CHECK (stock IS NULL OR stock > 0),
  reward_payload TEXT NOT NULL CHECK (json_valid(reward_payload)),
  UNIQUE(draw_id, position),
  CHECK ((probability_bps IS NOT NULL AND stock IS NULL) OR (probability_bps IS NULL AND stock IS NOT NULL))
);

CREATE TABLE admin_api_tokens (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 name TEXT NOT NULL CHECK(length(name) BETWEEN 1 AND 80),
 token_prefix TEXT NOT NULL,
 token_hash BYTEA NOT NULL UNIQUE CHECK(length(token_hash)=32),
 scopes_json TEXT NOT NULL CHECK(json_valid(scopes_json)),
 created_at TEXT NOT NULL,
 expires_at TEXT,
 last_used_at TEXT,
 revoked_at TEXT
);

CREATE TABLE affiliate_config_versions (
  id TEXT PRIMARY KEY,
  version BIGINT NOT NULL UNIQUE CHECK (version > 0),
  created_by TEXT REFERENCES users(id),
  created_at TEXT NOT NULL
);

CREATE TABLE audit_events (
  id TEXT PRIMARY KEY,
  actor_user_id TEXT REFERENCES users(id),
  action TEXT NOT NULL,
  target_type TEXT NOT NULL,
  target_id TEXT NOT NULL,
  detail TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL
);

CREATE TABLE backup_runs (
  id TEXT PRIMARY KEY,
  path TEXT NOT NULL,
  size_bytes BIGINT NOT NULL DEFAULT 0,
  status TEXT NOT NULL CHECK (status IN ('running','complete','failed')),
  error TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  completed_at TEXT
, source TEXT NOT NULL DEFAULT 'generated', actor_user_id TEXT REFERENCES users(id) ON DELETE SET NULL, idempotency_key TEXT NOT NULL DEFAULT '', request_fingerprint TEXT NOT NULL DEFAULT '', sha256 TEXT NOT NULL DEFAULT '', original_filename TEXT NOT NULL DEFAULT '', reason TEXT NOT NULL DEFAULT '');

CREATE TABLE balances (
  user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  txb_minor BIGINT NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL
);

CREATE TABLE browser_access_links (
  token_hash BYTEA PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  return_to TEXT NOT NULL,
  expires_at TEXT NOT NULL
);

CREATE TABLE "connection_scans" (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  idempotency_key TEXT NOT NULL CHECK (length(idempotency_key) BETWEEN 1 AND 128),
  request_fingerprint TEXT NOT NULL CHECK (length(request_fingerprint) BETWEEN 16 AND 128),
  provider_job_id TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'queued'
    CHECK (status IN ('queued','processing','succeeded','failed','pending_review')),
  progress_percent DOUBLE PRECISION NOT NULL DEFAULT 0
    CHECK (progress_percent >= 0 AND progress_percent <= 100),
  error_code TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  completed_at TEXT,
  expires_at TEXT NOT NULL,
  UNIQUE(user_id,idempotency_key)
);

CREATE TABLE coupon_grants (
  id TEXT PRIMARY KEY,
  coupon_id TEXT NOT NULL REFERENCES coupon_definitions(id),
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  source_type TEXT NOT NULL,
  source_id TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','consumed','expired')),
  use_count BIGINT NOT NULL DEFAULT 0 CHECK (use_count >= 0),
  created_at TEXT NOT NULL,
  consumed_at TEXT,
  UNIQUE(user_id, coupon_id, source_type, source_id)
);

CREATE TABLE database_admin_reviews (
  id TEXT PRIMARY KEY,
  actor_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  action TEXT NOT NULL CHECK (action IN ('insert','update','delete')),
  table_name TEXT NOT NULL,
  request_hash TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  consumed_at TEXT,
  created_at TEXT NOT NULL
);

CREATE TABLE "emby_accounts" (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
  base_username TEXT NOT NULL,
  remote_user_id TEXT UNIQUE,
  remote_username TEXT,
  candidate_username TEXT UNIQUE,
  status TEXT NOT NULL
    CHECK (status IN ('queued','provisioning','active','failed','pending_review')),
  setup_price_txb_minor BIGINT NOT NULL CHECK (setup_price_txb_minor >= 0),
  setup_attempt BIGINT NOT NULL DEFAULT 1 CHECK (setup_attempt >= 1),
  password_ciphertext TEXT NOT NULL DEFAULT '',
  password_context TEXT NOT NULL DEFAULT '',
  max_parental_rating BIGINT,
  create_attempted BIGINT NOT NULL DEFAULT 0 CHECK (create_attempted IN (0,1)),
  last_error TEXT NOT NULL DEFAULT '',
  provisioned_at TEXT,
  refunded_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  pending_preferences_json TEXT NOT NULL DEFAULT '{}'
    CHECK (json_valid(pending_preferences_json))
);

CREATE TABLE ledger_entries (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id),
  delta_txb_minor BIGINT NOT NULL,
  balance_after BIGINT NOT NULL,
  kind TEXT NOT NULL,
  reference_id TEXT NOT NULL,
  note TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  UNIQUE(kind, reference_id)
);

CREATE TABLE onboarding_content (
  kind TEXT PRIMARY KEY CHECK (kind IN ('welcome','agreements')),
  draft_json TEXT NOT NULL CHECK (json_valid(draft_json)),
  published_json TEXT NOT NULL CHECK (json_valid(published_json)),
  draft_revision BIGINT NOT NULL DEFAULT 1 CHECK (draft_revision >= 1),
  published_revision BIGINT NOT NULL DEFAULT 1 CHECK (published_revision >= 1),
  updated_at TEXT NOT NULL,
  published_at TEXT NOT NULL,
  updated_by TEXT REFERENCES users(id)
);

CREATE TABLE panel_entry_verification (
    telegram_id BIGINT PRIMARY KEY REFERENCES users(telegram_id) ON DELETE CASCADE,
    checked BIGINT NOT NULL DEFAULT 0 CHECK (checked IN (0,1))
);

CREATE TABLE "payment_orders" (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES users(id),
 provider TEXT NOT NULL CHECK(provider IN ('ezpay','bepusdt','stars','builtin_usdt')),
 status TEXT NOT NULL CHECK(status IN ('creating','pending','paid','expired','failed','refunded')),
 txb_minor BIGINT NOT NULL CHECK(txb_minor>0),
 payable_amount TEXT NOT NULL, payable_currency TEXT NOT NULL, rate_snapshot TEXT NOT NULL,
 provider_trade_id TEXT UNIQUE, provider_charge_id TEXT UNIQUE,
 payment_url TEXT, qr_payload TEXT, provider_payload TEXT NOT NULL DEFAULT '{}',
 expires_at TEXT NOT NULL, paid_at TEXT, refunded_at TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
 method_id TEXT NOT NULL DEFAULT '', provider_rail TEXT NOT NULL DEFAULT '',
 rate_direction TEXT NOT NULL DEFAULT 'currency_per_txb' CHECK(rate_direction IN ('currency_per_txb','txb_per_currency')),
 receiving_address TEXT, actual_crypto_amount TEXT, actual_crypto_currency TEXT,
 cancelled_at TEXT, cancel_reason TEXT NOT NULL DEFAULT '',
 provider_cancel_status TEXT NOT NULL DEFAULT '' CHECK(provider_cancel_status IN ('','unsupported','requested','cancelled','failed','pending_review'))
);

CREATE TABLE provider_operations (
  id TEXT PRIMARY KEY,
  actor_user_id TEXT NOT NULL REFERENCES users(id),
  owner_user_id TEXT REFERENCES users(id),
  kind TEXT NOT NULL CHECK (length(kind) BETWEEN 1 AND 80),
  idempotency_key TEXT NOT NULL CHECK (length(idempotency_key) BETWEEN 1 AND 128),
  request_fingerprint TEXT NOT NULL CHECK (length(request_fingerprint) BETWEEN 16 AND 128),
  status TEXT NOT NULL DEFAULT 'queued'
    CHECK (status IN ('queued','processing','succeeded','failed','compensated','pending_review','partial')),
  attempts BIGINT NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  attempt_started_at TEXT,
  provider_reference TEXT NOT NULL DEFAULT '',
  error_code TEXT NOT NULL DEFAULT '',
  result_json TEXT NOT NULL DEFAULT '{}'
    CHECK (json_valid(result_json) AND json_type(result_json)='object'),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  completed_at TEXT,
  UNIQUE(actor_user_id,kind,idempotency_key)
);

CREATE TABLE questionnaire_imports (
  id TEXT PRIMARY KEY,
  questionnaire_id TEXT NOT NULL REFERENCES questionnaires(id) ON DELETE CASCADE,
  status TEXT NOT NULL CHECK (status IN ('preview','queued','processing','settled','failed')),
  raw_csv BYTEA NOT NULL,
  delimiter TEXT NOT NULL CHECK (length(delimiter) = 1),
  headers_json TEXT NOT NULL,
  sample_rows_json TEXT NOT NULL,
  data_row_count BIGINT NOT NULL CHECK (data_row_count >= 0),
  malformed_row_count BIGINT NOT NULL CHECK (malformed_row_count >= 0),
  code_column TEXT,
  analysis_json TEXT,
  report_json TEXT,
  idempotency_key TEXT NOT NULL,
  last_error TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(questionnaire_id, idempotency_key)
);

CREATE TABLE questionnaire_participants (
  id TEXT PRIMARY KEY,
  questionnaire_id TEXT NOT NULL REFERENCES questionnaires(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  validation_code TEXT NOT NULL UNIQUE,
  awarded_at TEXT,
  created_at TEXT NOT NULL,
  UNIQUE(questionnaire_id, user_id)
);

CREATE TABLE sessions (
  token_hash BYTEA PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at TEXT NOT NULL,
  created_at TEXT NOT NULL
, scope TEXT NOT NULL DEFAULT 'miniapp' CHECK (scope IN ('miniapp','browser')), renewed_at TEXT NOT NULL DEFAULT '');

CREATE TABLE settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  encrypted BIGINT NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL,
  updated_by TEXT REFERENCES users(id)
);

CREATE TABLE txb_limits (
  singleton BIGINT PRIMARY KEY CHECK (singleton=1),
  minimum_txb_minor BIGINT NOT NULL CHECK (minimum_txb_minor > 0),
  maximum_txb_minor BIGINT NOT NULL CHECK (maximum_txb_minor >= minimum_txb_minor),
  updated_by TEXT REFERENCES users(id) ON DELETE SET NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE user_disabled_squads (
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 squad_uuid TEXT NOT NULL,
 PRIMARY KEY(user_id,squad_uuid)
);

CREATE TABLE user_notification_events (
  event_key TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  source_kind TEXT NOT NULL,
  source_id TEXT NOT NULL,
  kind TEXT NOT NULL,
  payload_json TEXT NOT NULL CHECK (json_valid(payload_json)),
  gate_key TEXT NOT NULL DEFAULT '',
  queued_at TEXT,
  created_at TEXT NOT NULL,
  CHECK (gate_key <> '' OR queued_at IS NOT NULL)
);

CREATE TABLE user_preferences (
  user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  notify_combos BIGINT NOT NULL DEFAULT 1 CHECK (notify_combos IN (0,1)),
  notify_traffic BIGINT NOT NULL DEFAULT 1 CHECK (notify_traffic IN (0,1)),
  notify_money BIGINT NOT NULL DEFAULT 1 CHECK (notify_money IN (0,1)),
  notify_activity BIGINT NOT NULL DEFAULT 1 CHECK (notify_activity IN (0,1)),
  notify_account BIGINT NOT NULL DEFAULT 1 CHECK (notify_account IN (0,1)),
  show_referral_username BIGINT NOT NULL DEFAULT 1 CHECK (show_referral_username IN (0,1)),
  show_around_tx BIGINT NOT NULL DEFAULT 0 CHECK (show_around_tx IN (0,1)),
  show_activity BIGINT NOT NULL DEFAULT 0 CHECK (show_activity IN (0,1)),
  include_node_prices BIGINT NOT NULL DEFAULT 1 CHECK (include_node_prices IN (0,1))
);

CREATE TABLE xnode_config_snippets (
 config_id TEXT NOT NULL REFERENCES xnode_configs(id) ON DELETE CASCADE,
 snippet_id TEXT NOT NULL REFERENCES xnode_snippets(id), position BIGINT NOT NULL,
 PRIMARY KEY(config_id,snippet_id), UNIQUE(config_id,position)
);

CREATE TABLE xnode_identities (
 proxy_user_id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
 user_id TEXT UNIQUE REFERENCES users(id), service_name TEXT UNIQUE,
 credentials_cipher TEXT NOT NULL, subscription_hash BYTEA UNIQUE,
 subscription_cipher TEXT NOT NULL DEFAULT '', current_period_id TEXT,
 legacy_user_id TEXT, lifetime_base BIGINT NOT NULL DEFAULT 0 CHECK(lifetime_base>=0),
 access_disabled BIGINT NOT NULL DEFAULT 0 CHECK(access_disabled IN(0,1)), created_at TEXT NOT NULL,
 CHECK((user_id IS NOT NULL AND service_name IS NULL) OR (user_id IS NULL AND service_name IS NOT NULL))
);

CREATE TABLE xnode_member_diy (
 user_id TEXT PRIMARY KEY REFERENCES users(id),
 revision BIGINT NOT NULL CHECK(revision>=1 AND revision<=9007199254740991),
 cipher TEXT NOT NULL
);

CREATE TABLE "xnode_nodes" (
 id TEXT PRIMARY KEY, name TEXT NOT NULL, country_code TEXT NOT NULL DEFAULT '',
 provider_name TEXT NOT NULL DEFAULT '', multiplier_fp BIGINT NOT NULL DEFAULT 1000000 CHECK(multiplier_fp>=0),
 enabled BIGINT NOT NULL DEFAULT 1 CHECK(enabled IN (0,1)),
 config_id TEXT NOT NULL REFERENCES xnode_configs(id), settings_cipher TEXT NOT NULL,
 token_hash BYTEA UNIQUE CHECK(token_hash IS NULL OR length(token_hash)=32), agent_id TEXT UNIQUE,
 desired_revision BIGINT NOT NULL DEFAULT 0, accepted_revision BIGINT NOT NULL DEFAULT 0,
 applied_revision BIGINT NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);

CREATE TABLE abuse_notification_deliveries (
  record_id TEXT NOT NULL REFERENCES abuse_records(id) ON DELETE CASCADE, recipient_telegram_id BIGINT NOT NULL,
  kind TEXT NOT NULL, delivered_at TEXT, created_at TEXT NOT NULL, PRIMARY KEY(record_id,recipient_telegram_id,kind)
);

CREATE TABLE abuse_record_nodes (record_id TEXT NOT NULL REFERENCES abuse_records(id) ON DELETE CASCADE, node_uuid TEXT NOT NULL, PRIMARY KEY(record_id,node_uuid));

CREATE TABLE abuse_record_reasons (record_id TEXT NOT NULL REFERENCES abuse_records(id) ON DELETE CASCADE, name TEXT NOT NULL, PRIMARY KEY(record_id,name));

CREATE TABLE abuse_temp_bans (
  record_id TEXT PRIMARY KEY REFERENCES abuse_records(id) ON DELETE CASCADE, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at TEXT NOT NULL, restore_queued_at TEXT, restored_at TEXT, created_at TEXT NOT NULL
);

CREATE TABLE activity_raffle_tickets (
  id TEXT PRIMARY KEY,
  draw_id TEXT NOT NULL REFERENCES activity_lucky_draws(id),
  user_id TEXT NOT NULL REFERENCES users(id),
  chat_id BIGINT NOT NULL,
  message_id BIGINT NOT NULL,
  fee_minor BIGINT NOT NULL CHECK (fee_minor > 0),
  reserve_minor BIGINT NOT NULL DEFAULT 0 CHECK (reserve_minor >= 0),
  configuration_revision BIGINT NOT NULL CHECK (configuration_revision > 0),
  status TEXT NOT NULL CHECK (status IN ('active','refunded','settled')),
  prize_id TEXT,
  result_id TEXT REFERENCES activity_draw_results(id),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(chat_id, message_id)
);

CREATE TABLE admin_coupon_discard_commands (
  actor_user_id TEXT NOT NULL REFERENCES users(id), idempotency_key TEXT NOT NULL,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  grant_id TEXT NOT NULL REFERENCES coupon_grants(id) ON DELETE CASCADE,
  created_at TEXT NOT NULL, PRIMARY KEY(actor_user_id, idempotency_key)
);

CREATE TABLE admin_temporary_bans (
  user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  actor_user_id TEXT NOT NULL REFERENCES users(id), reason TEXT NOT NULL,
  expires_at TEXT NOT NULL, ban_operation_id TEXT NOT NULL REFERENCES provider_operations(id),
  unban_operation_id TEXT REFERENCES provider_operations(id), unban_reason TEXT,
  restored_at TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);

CREATE TABLE affiliate_config_current (
  singleton BIGINT PRIMARY KEY CHECK (singleton=1),
  config_id TEXT NOT NULL REFERENCES affiliate_config_versions(id)
);

CREATE TABLE affiliate_tiers (
  id TEXT PRIMARY KEY,
  config_id TEXT NOT NULL REFERENCES affiliate_config_versions(id),
  position BIGINT NOT NULL CHECK (position >= 0),
  name TEXT NOT NULL,
  success_threshold BIGINT NOT NULL CHECK (success_threshold >= 0),
  enabled BIGINT NOT NULL CHECK (enabled IN (0,1)),
  commission_enabled BIGINT NOT NULL CHECK (commission_enabled IN (0,1)),
  commission_bps BIGINT NOT NULL CHECK (commission_bps BETWEEN 0 AND 10000),
  reward_kind TEXT NOT NULL CHECK (reward_kind IN ('none','coupon','txb','subscription_extension')),
  reward_coupon_id TEXT REFERENCES coupon_definitions(id),
  reward_txb_minor BIGINT CHECK (reward_txb_minor > 0),
  reward_extension_days BIGINT CHECK (reward_extension_days BETWEEN 1 AND 3650),
  CHECK (
    (reward_kind='none' AND reward_coupon_id IS NULL AND reward_txb_minor IS NULL AND reward_extension_days IS NULL) OR
    (reward_kind='coupon' AND reward_coupon_id IS NOT NULL AND reward_txb_minor IS NULL AND reward_extension_days IS NULL) OR
    (reward_kind='txb' AND reward_coupon_id IS NULL AND reward_txb_minor IS NOT NULL AND reward_extension_days IS NULL) OR
    (reward_kind='subscription_extension' AND reward_coupon_id IS NULL AND reward_txb_minor IS NULL AND reward_extension_days IS NOT NULL)
  ),
  UNIQUE(config_id,position)
);

CREATE TABLE backup_uploads (
  id TEXT PRIMARY KEY,
  backup_run_id TEXT NOT NULL UNIQUE REFERENCES backup_runs(id) ON DELETE CASCADE,
  actor_user_id TEXT NOT NULL REFERENCES users(id),
  idempotency_key TEXT NOT NULL CHECK (length(idempotency_key) BETWEEN 1 AND 128),
  expected_sha256 TEXT NOT NULL DEFAULT '',
  actual_sha256 TEXT NOT NULL DEFAULT '',
  temporary_path TEXT NOT NULL,
  final_path TEXT NOT NULL,
  size_bytes BIGINT NOT NULL DEFAULT 0 CHECK (size_bytes >= 0),
  status TEXT NOT NULL CHECK (status IN ('receiving','validating','publishing','complete','failed')),
  last_error TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  completed_at TEXT,
  UNIQUE(actor_user_id,idempotency_key)
);

CREATE TABLE connection_ip_blocks (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  node_uuid TEXT NOT NULL,
  ip_digest TEXT NOT NULL CHECK (length(ip_digest) = 64),
  sealed_ip TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'blocking'
    CHECK (status IN ('blocking','active','unblocking','pending_review')),
  block_operation_id TEXT REFERENCES provider_operations(id) ON DELETE SET NULL,
  unblock_operation_id TEXT REFERENCES provider_operations(id) ON DELETE SET NULL,
  expiry_job_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  UNIQUE(user_id,node_uuid,ip_digest)
);

CREATE TABLE coupon_grant_discards (
  grant_id TEXT PRIMARY KEY REFERENCES coupon_grants(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  discarded_at TEXT NOT NULL
);

CREATE TABLE coupon_redemptions (
  id TEXT PRIMARY KEY,
  coupon_id TEXT NOT NULL REFERENCES coupon_definitions(id),
  grant_id TEXT REFERENCES coupon_grants(id),
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  balance_delta_minor BIGINT NOT NULL DEFAULT 0,
  balance_after_minor BIGINT NOT NULL,
  idempotency_key TEXT NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE(user_id, idempotency_key)
);

CREATE TABLE coupon_uses (
  id TEXT PRIMARY KEY,
  coupon_id TEXT NOT NULL REFERENCES coupon_definitions(id),
  grant_id TEXT NOT NULL REFERENCES coupon_grants(id),
  user_id TEXT NOT NULL REFERENCES users(id),
  purchase_id TEXT NOT NULL,
  gross_price_minor BIGINT NOT NULL CHECK (gross_price_minor >= 0),
  discount_minor BIGINT NOT NULL CHECK (discount_minor >= 0),
  net_price_minor BIGINT NOT NULL CHECK (net_price_minor >= 0),
  created_at TEXT NOT NULL,
  UNIQUE(purchase_id),
  UNIQUE(grant_id, purchase_id)
);

CREATE TABLE courtesy_credits (
  id TEXT PRIMARY KEY,
  payment_order_id TEXT NOT NULL UNIQUE REFERENCES payment_orders(id) ON DELETE CASCADE,
  actor_user_id TEXT NOT NULL REFERENCES users(id),
  txb_minor BIGINT NOT NULL CHECK (txb_minor > 0),
  ledger_entry_id TEXT NOT NULL UNIQUE REFERENCES ledger_entries(id),
  reason TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE "emby_account_disabled_folders" (
  account_id TEXT NOT NULL REFERENCES "emby_accounts"(id) ON DELETE CASCADE,
  folder_id TEXT NOT NULL,
  PRIMARY KEY (account_id,folder_id)
);

CREATE TABLE "maintenance_runs" (
  id TEXT PRIMARY KEY,
  local_date TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('running','succeeded','failed')),
  lease_owner TEXT NOT NULL,
  lease_expires_at TEXT NOT NULL,
  backup_run_id TEXT REFERENCES backup_runs(id) ON DELETE SET NULL,
  cleanup_counts_json TEXT NOT NULL DEFAULT '{}'
    CHECK (json_valid(cleanup_counts_json) AND json_type(cleanup_counts_json)='object'),
  error_code TEXT NOT NULL DEFAULT '',
  started_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  completed_at TEXT
);

CREATE TABLE node_compensation_events (
  id TEXT PRIMARY KEY,
  node_uuid TEXT NOT NULL,
  node_name TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('observing','pending_review','queued','dismissed','ineligible')),
  offline_observed_at TEXT NOT NULL,
  recovered_observed_at TEXT,
  observed_duration_seconds BIGINT CHECK (observed_duration_seconds IS NULL OR observed_duration_seconds >= 0),
  threshold_minutes BIGINT NOT NULL CHECK (threshold_minutes BETWEEN 1 AND 5256000),
  multiplier_bps BIGINT NOT NULL CHECK (multiplier_bps BETWEEN 100 AND 1000000),
  proposed_extension_minutes BIGINT CHECK (proposed_extension_minutes IS NULL OR proposed_extension_minutes BETWEEN 0 AND 5256000),
  final_extension_minutes BIGINT CHECK (final_extension_minutes IS NULL OR final_extension_minutes BETWEEN 1 AND 5256000),
  capped BIGINT NOT NULL DEFAULT 0 CHECK (capped IN (0,1)),
  frozen_recipient_count BIGINT NOT NULL DEFAULT 0 CHECK (frozen_recipient_count >= 0),
  eligible_recipient_count BIGINT CHECK (eligible_recipient_count IS NULL OR eligible_recipient_count >= 0),
  skipped_recipient_count BIGINT CHECK (skipped_recipient_count IS NULL OR skipped_recipient_count >= 0),
  ineligible_reason TEXT CHECK (ineligible_reason IS NULL OR ineligible_reason IN ('node_disabled','below_threshold','no_recipients','computed_zero')),
  reviewed_by TEXT REFERENCES users(id),
  reviewed_at TEXT,
  review_reason TEXT,
  review_idempotency_key TEXT,
  review_fingerprint TEXT,
  provider_operation_id TEXT REFERENCES provider_operations(id),
  revision BIGINT NOT NULL DEFAULT 0 CHECK (revision >= 0),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE provider_operation_items (
  operation_id TEXT NOT NULL REFERENCES provider_operations(id) ON DELETE CASCADE,
  item_key TEXT NOT NULL CHECK (length(item_key) BETWEEN 1 AND 160),
  target_type TEXT NOT NULL CHECK (length(target_type) BETWEEN 1 AND 80),
  target_id TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'queued'
    CHECK (status IN ('queued','processing','succeeded','failed','compensated','pending_review')),
  provider_reference TEXT NOT NULL DEFAULT '',
  error_code TEXT NOT NULL DEFAULT '',
  result_json TEXT NOT NULL DEFAULT '{}'
    CHECK (json_valid(result_json) AND json_type(result_json)='object'),
  attempt_started_at TEXT,
  completed_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY(operation_id,item_key)
);

CREATE TABLE provider_operation_replays (
  id TEXT PRIMARY KEY,
  operation_id TEXT NOT NULL REFERENCES provider_operations(id) ON DELETE CASCADE,
  actor_user_id TEXT NOT NULL REFERENCES users(id),
  idempotency_key TEXT NOT NULL,
  request_fingerprint TEXT NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE(operation_id,idempotency_key,request_fingerprint)
);

CREATE TABLE purchases (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id),
  combo_id TEXT NOT NULL REFERENCES combos(id),
  charged_txb_minor BIGINT NOT NULL CHECK (charged_txb_minor >= 0),
  valid_from TEXT NOT NULL,
  valid_until TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('activating','active','queued','expired','cancelled','failed')),
  traffic_reset_phase TEXT NOT NULL DEFAULT 'pending' CHECK (traffic_reset_phase IN ('pending','quiesced','reset')),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
, coupon_grant_id TEXT REFERENCES coupon_grants(id), gross_price_txb_minor BIGINT CHECK (gross_price_txb_minor IS NULL OR gross_price_txb_minor >= 0), coupon_discount_txb_minor BIGINT NOT NULL DEFAULT 0 CHECK (coupon_discount_txb_minor >= 0), idempotency_key TEXT, request_fingerprint TEXT NOT NULL DEFAULT '', renewal_batch_id TEXT, renewal_index BIGINT, auto_renew_enabled BIGINT NOT NULL DEFAULT 0
  CHECK (auto_renew_enabled IN (0,1)), auto_renew_failure_reason TEXT NOT NULL DEFAULT '', auto_renew_failed_at TEXT, recurring_discount_attached BIGINT NOT NULL DEFAULT 0
  CHECK (recurring_discount_attached IN (0,1)), auto_renew_source_purchase_id TEXT
  REFERENCES purchases(id), core_gross_txb_minor BIGINT NOT NULL DEFAULT 0
  CHECK (core_gross_txb_minor >= 0), entitlement_traffic_limit_bytes BIGINT
  CHECK (entitlement_traffic_limit_bytes IS NULL OR entitlement_traffic_limit_bytes > 0), entitlement_reset_strategy TEXT
  CHECK (entitlement_reset_strategy IS NULL OR entitlement_reset_strategy IN ('DAY','WEEK','MONTH_ROLLING')), entitlement_squad_uuids TEXT
  CHECK (entitlement_squad_uuids IS NULL OR (json_valid(entitlement_squad_uuids) AND json_type(entitlement_squad_uuids)='array')), entitlement_addon_squad_uuids TEXT
  CHECK (entitlement_addon_squad_uuids IS NULL OR (json_valid(entitlement_addon_squad_uuids) AND json_type(entitlement_addon_squad_uuids)='array')), reward_renewal_price_minor BIGINT CHECK (reward_renewal_price_minor IS NULL OR reward_renewal_price_minor >= 0), reward_rollover_min_remaining_bps BIGINT CHECK (reward_rollover_min_remaining_bps IS NULL OR reward_rollover_min_remaining_bps BETWEEN 0 AND 10000), reward_traffic_renewal BIGINT NOT NULL DEFAULT 1 CHECK (reward_traffic_renewal IN (0,1)), reward_renewal_traffic_limit_bytes BIGINT
  CHECK (reward_renewal_traffic_limit_bytes IS NULL OR reward_renewal_traffic_limit_bytes > 0));

CREATE TABLE refunds (
  id TEXT PRIMARY KEY,
  payment_order_id TEXT NOT NULL UNIQUE REFERENCES payment_orders(id),
  actor_user_id TEXT REFERENCES users(id),
  txb_minor BIGINT NOT NULL CHECK (txb_minor > 0),
  reason TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('completed')),
  created_at TEXT NOT NULL
);

CREATE TABLE restore_jobs (
  id TEXT PRIMARY KEY,
  backup_run_id TEXT NOT NULL REFERENCES backup_runs(id),
  actor_user_id TEXT REFERENCES users(id),
  status TEXT NOT NULL CHECK (status IN ('staging','ready','applying','complete','failed','rolled_back')),
  staged_path TEXT NOT NULL,
  rescue_path TEXT NOT NULL,
  source_sha256 TEXT NOT NULL,
  error TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  completed_at TEXT
, request_actor_id TEXT NOT NULL DEFAULT '', idempotency_key TEXT NOT NULL DEFAULT ''
  CHECK (idempotency_key = '' OR length(idempotency_key) BETWEEN 1 AND 128), request_fingerprint TEXT NOT NULL DEFAULT ''
  CHECK (request_fingerprint = '' OR length(request_fingerprint) BETWEEN 16 AND 128));

CREATE TABLE telegram_pm_conversations (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    chat_id BIGINT NOT NULL CHECK (chat_id<0),
    topic_id BIGINT CHECK (topic_id>1),
    profile_message_id BIGINT CHECK (profile_message_id>0),
    profile_state TEXT NOT NULL DEFAULT 'missing' CHECK (profile_state IN ('missing','ready','pending_review')),
    topic_state TEXT NOT NULL DEFAULT 'new' CHECK (topic_state IN ('new','creating','ready','pending_review')),
    topic_operation_id TEXT REFERENCES provider_operations(id) ON DELETE SET NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (user_id,chat_id)
);

CREATE TABLE telegram_pm_payloads (
    operation_id TEXT PRIMARY KEY REFERENCES provider_operations(id) ON DELETE CASCADE,
    encrypted_payload TEXT NOT NULL,
    expires_at BIGINT NOT NULL
);

CREATE TABLE telegram_pm_read_status (
    operation_id TEXT PRIMARY KEY REFERENCES provider_operations(id) ON DELETE CASCADE,
    read_at TEXT NOT NULL,
    source TEXT NOT NULL CHECK (source IN ('explicit','reply'))
);

CREATE TABLE webhook_events (
  id TEXT PRIMARY KEY,
  provider TEXT NOT NULL,
  dedupe_key TEXT NOT NULL,
  order_id TEXT NOT NULL REFERENCES payment_orders(id),
  payload_hash TEXT NOT NULL,
  received_at TEXT NOT NULL,
  processed_at TEXT,
  UNIQUE(provider, dedupe_key)
);

CREATE TABLE "xnode_customizations" (
 proxy_user_id BIGINT NOT NULL REFERENCES xnode_identities(proxy_user_id),
 user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
 node_id TEXT NOT NULL REFERENCES xnode_nodes(id), outbounds_cipher TEXT NOT NULL, routing_cipher TEXT NOT NULL,
 revision BIGINT NOT NULL DEFAULT 1, updated_at TEXT NOT NULL, PRIMARY KEY(proxy_user_id,node_id)
);

CREATE TABLE xnode_endpoints (
 id TEXT PRIMARY KEY, node_id TEXT NOT NULL REFERENCES xnode_nodes(id), inbound_tag TEXT NOT NULL,
 name TEXT NOT NULL, address TEXT NOT NULL, port BIGINT NOT NULL CHECK(port BETWEEN 1 AND 65535),
 enabled BIGINT NOT NULL DEFAULT 1 CHECK(enabled IN(0,1)), hidden BIGINT NOT NULL DEFAULT 0 CHECK(hidden IN(0,1)),
 overrides_cipher TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);

CREATE TABLE xnode_firewall_cursors(node_id TEXT NOT NULL REFERENCES xnode_nodes(id) ON DELETE CASCADE,namespace TEXT NOT NULL,sequence BIGINT NOT NULL,PRIMARY KEY(node_id,namespace));

CREATE TABLE xnode_firewall_exemptions(
 id TEXT PRIMARY KEY,user_id TEXT REFERENCES users(id),idempotency_key TEXT NOT NULL,ip TEXT NOT NULL,
 price_minor BIGINT NOT NULL,hours BIGINT NOT NULL,until_at TEXT NOT NULL,created_at TEXT NOT NULL,
 issued_version BIGINT NOT NULL,released_version BIGINT NOT NULL DEFAULT 0,ledger_id TEXT REFERENCES ledger_entries(id), grant_kind TEXT NOT NULL DEFAULT 'member'
 CHECK(grant_kind IN ('member','admin')),
 UNIQUE(user_id,idempotency_key)
);

CREATE TABLE xnode_group_bindings (
 group_id TEXT NOT NULL REFERENCES xnode_groups(id) ON DELETE CASCADE,
 node_id TEXT NOT NULL REFERENCES xnode_nodes(id), inbound_tag TEXT NOT NULL,
 PRIMARY KEY(group_id,node_id,inbound_tag)
);

CREATE TABLE "xnode_imported_history" (
 proxy_user_id BIGINT NOT NULL REFERENCES xnode_identities(proxy_user_id),
 node_id TEXT NOT NULL, date TEXT NOT NULL,
 raw_bytes BIGINT NOT NULL CHECK(raw_bytes>=0), weighted_bytes BIGINT NOT NULL CHECK(weighted_bytes>=0),
 PRIMARY KEY(proxy_user_id,node_id,date)
);

CREATE TABLE xnode_report_cursors (
 agent_id TEXT PRIMARY KEY, node_id TEXT NOT NULL REFERENCES xnode_nodes(id),
 last_sequence BIGINT NOT NULL DEFAULT 0, last_digest TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL
);

CREATE TABLE xnode_routing_plans(
 proxy_user_id BIGINT PRIMARY KEY REFERENCES xnode_identities(proxy_user_id) ON DELETE CASCADE,
 cipher TEXT NOT NULL, previous_cipher TEXT NOT NULL DEFAULT '', legacy_cipher TEXT NOT NULL DEFAULT '',
 revision BIGINT NOT NULL CHECK(revision>0), phase TEXT NOT NULL CHECK(phase IN('revoking','applying','enabling','active','disabled')),
 operation_id TEXT NOT NULL REFERENCES provider_operations(id), reason TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL
);

CREATE TABLE xnode_service_bindings (
 proxy_user_id BIGINT NOT NULL REFERENCES xnode_identities(proxy_user_id),
 node_id TEXT NOT NULL REFERENCES xnode_nodes(id), inbound_tag TEXT NOT NULL,
 valid_until TEXT NOT NULL, PRIMARY KEY(proxy_user_id,node_id,inbound_tag)
);

CREATE TABLE activity_extension_credits (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  days BIGINT NOT NULL CHECK (days BETWEEN 1 AND 3650),
  source_type TEXT NOT NULL,
  source_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  consumed_at TEXT,
  consumed_by_purchase_id TEXT REFERENCES purchases(id), hours BIGINT CHECK (hours IS NULL OR hours > 0),
  UNIQUE(source_type, source_id)
);

CREATE TABLE activity_reward_squad_access (
  purchase_id TEXT NOT NULL REFERENCES purchases(id) ON DELETE CASCADE,
  squad_uuid TEXT NOT NULL,
  source_result_id TEXT NOT NULL REFERENCES activity_draw_results(id),
  PRIMARY KEY(purchase_id,squad_uuid,source_result_id)
);

CREATE TABLE affiliate_settlements (
  id TEXT PRIMARY KEY,
  invited_user_id TEXT NOT NULL UNIQUE REFERENCES users(id),
  inviter_user_id TEXT NOT NULL REFERENCES users(id),
  payment_order_id TEXT NOT NULL UNIQUE REFERENCES payment_orders(id),
  config_id TEXT NOT NULL REFERENCES affiliate_config_versions(id),
  tier_id TEXT NOT NULL REFERENCES affiliate_tiers(id),
  tier_name TEXT NOT NULL,
  commission_bps BIGINT NOT NULL CHECK (commission_bps BETWEEN 0 AND 10000),
  topup_txb_minor BIGINT NOT NULL CHECK (topup_txb_minor > 0),
  commission_txb_minor BIGINT NOT NULL CHECK (commission_txb_minor >= 0),
  settled_at TEXT NOT NULL
);

CREATE TABLE ip_lookup_allowances (
    purchase_id TEXT PRIMARY KEY REFERENCES purchases(id) ON DELETE CASCADE,
    total BIGINT NOT NULL CHECK(total >= 0),
    used BIGINT NOT NULL DEFAULT 0 CHECK(used >= 0 AND used <= total)
);

CREATE TABLE node_compensation_event_recipients (
  event_id TEXT NOT NULL REFERENCES node_compensation_events(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL REFERENCES users(id),
  PRIMARY KEY(event_id,user_id)
);

CREATE TABLE node_compensation_event_squads (
  event_id TEXT NOT NULL REFERENCES node_compensation_events(id) ON DELETE CASCADE,
  squad_uuid TEXT NOT NULL,
  squad_name TEXT NOT NULL,
  PRIMARY KEY(event_id,squad_uuid)
);

CREATE TABLE node_compensation_node_state (
  node_uuid TEXT PRIMARY KEY,
  node_name TEXT NOT NULL,
  is_connected BIGINT NOT NULL CHECK (is_connected IN (0,1)),
  is_disabled BIGINT NOT NULL CHECK (is_disabled IN (0,1)),
  open_event_id TEXT REFERENCES node_compensation_events(id),
  last_observed_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE purchase_addon_adjustments (
  id TEXT PRIMARY KEY,
  purchase_id TEXT NOT NULL REFERENCES purchases(id) ON DELETE CASCADE,
  idempotency_key TEXT NOT NULL CHECK (length(idempotency_key) BETWEEN 1 AND 128),
  request_fingerprint TEXT NOT NULL CHECK (length(request_fingerprint) BETWEEN 16 AND 128),
  charged_txb_minor BIGINT NOT NULL CHECK (charged_txb_minor >= 0),
  created_at TEXT NOT NULL,
  UNIQUE(purchase_id,idempotency_key)
);

CREATE TABLE purchase_addons (
  purchase_id TEXT NOT NULL REFERENCES purchases(id) ON DELETE CASCADE,
  group_id TEXT NOT NULL,
  charged_txb_minor BIGINT NOT NULL DEFAULT 0 CHECK (charged_txb_minor >= 0),
  PRIMARY KEY (purchase_id, group_id)
);

CREATE TABLE "purchase_rollovers" (
  purchase_id TEXT PRIMARY KEY REFERENCES purchases(id) ON DELETE CASCADE,
  status TEXT NOT NULL CHECK (status IN ('pending','processing','calculated','credited','zero','exception')),
  traffic_limit_bytes BIGINT NOT NULL CHECK (traffic_limit_bytes >= 0),
  used_traffic_bytes BIGINT,
  remaining_traffic_bytes BIGINT,
  minimum_remaining_bps BIGINT NOT NULL CHECK (minimum_remaining_bps BETWEEN 0 AND 10000),
  net_paid_txb_minor BIGINT NOT NULL CHECK (net_paid_txb_minor >= 0),
  credited_txb_minor BIGINT NOT NULL DEFAULT 0 CHECK (credited_txb_minor >= 0),
  exception_code TEXT NOT NULL DEFAULT '',
  attempts BIGINT NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  completed_at TEXT,
  allocated_traffic_bytes BIGINT,
  eligible_unused_bytes BIGINT,
  algorithm_version TEXT NOT NULL DEFAULT ''
);

CREATE TABLE renewal_batches (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  source_purchase_id TEXT NOT NULL REFERENCES purchases(id),
  idempotency_key TEXT NOT NULL,
  request_fingerprint TEXT NOT NULL,
  term_count BIGINT NOT NULL CHECK (term_count BETWEEN 1 AND 6),
  charged_txb_minor BIGINT NOT NULL CHECK (charged_txb_minor >= 0),
  created_at TEXT NOT NULL,
  UNIQUE(user_id, idempotency_key)
);

CREATE TABLE xnode_periods (
 id TEXT PRIMARY KEY, proxy_user_id BIGINT NOT NULL REFERENCES xnode_identities(proxy_user_id),
 purchase_id TEXT REFERENCES purchases(id), started_at TEXT NOT NULL, ended_at TEXT,
 limit_bytes BIGINT NOT NULL CHECK(limit_bytes>=0), strategy TEXT NOT NULL,
 last_reset_at TEXT, used_bytes BIGINT NOT NULL DEFAULT 0 CHECK(used_bytes>=0),
 status TEXT NOT NULL DEFAULT 'open' CHECK(status IN('open','closing','closed','review'))
);

CREATE TABLE affiliate_tier_awards (
  id TEXT PRIMARY KEY,
  inviter_user_id TEXT NOT NULL REFERENCES users(id),
  tier_id TEXT NOT NULL REFERENCES affiliate_tiers(id),
  settlement_id TEXT NOT NULL UNIQUE REFERENCES affiliate_settlements(id),
  tier_name TEXT NOT NULL,
  reward_kind TEXT NOT NULL CHECK (reward_kind IN ('none','coupon','txb','subscription_extension')),
  reward_description TEXT NOT NULL,
  reward_coupon_id TEXT REFERENCES coupon_definitions(id),
  reward_txb_minor BIGINT,
  reward_extension_days BIGINT,
  awarded_at TEXT NOT NULL,
  CHECK (
    (reward_kind='none' AND reward_coupon_id IS NULL AND reward_txb_minor IS NULL AND reward_extension_days IS NULL) OR
    (reward_kind='coupon' AND reward_coupon_id IS NOT NULL AND reward_txb_minor IS NULL AND reward_extension_days IS NULL) OR
    (reward_kind='txb' AND reward_coupon_id IS NULL AND reward_txb_minor IS NOT NULL AND reward_extension_days IS NULL) OR
    (reward_kind='subscription_extension' AND reward_coupon_id IS NULL AND reward_txb_minor IS NULL AND reward_extension_days IS NOT NULL)
  ),
  UNIQUE(inviter_user_id,tier_id)
);

CREATE TABLE "ip_lookup_checks" (
 id TEXT NOT NULL UNIQUE,
 operation_id TEXT PRIMARY KEY REFERENCES provider_operations(id) ON DELETE CASCADE,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 report_id TEXT REFERENCES ip_lookup_reports(id) ON DELETE SET NULL,
 allowance_id TEXT REFERENCES ip_lookup_allowances(purchase_id) ON DELETE SET NULL,
 charge_minor BIGINT NOT NULL CHECK(charge_minor >= 0),
 cached BIGINT NOT NULL CHECK(cached IN (0,1)),
 refunded BIGINT NOT NULL DEFAULT 0 CHECK(refunded IN (0,1)),
 quota_consumed BIGINT NOT NULL DEFAULT 0 CHECK(quota_consumed BETWEEN 0 AND 1000000)
);

CREATE TABLE xnode_daily_usage (
 proxy_user_id BIGINT NOT NULL REFERENCES xnode_identities(proxy_user_id),
 node_id TEXT NOT NULL REFERENCES xnode_nodes(id), period_id TEXT NOT NULL REFERENCES xnode_periods(id),
 date TEXT NOT NULL, upload_bytes BIGINT NOT NULL DEFAULT 0 CHECK(upload_bytes>=0),
 download_bytes BIGINT NOT NULL DEFAULT 0 CHECK(download_bytes>=0),
 weighted_bytes BIGINT NOT NULL DEFAULT 0 CHECK(weighted_bytes>=0),
 weight_remainder BIGINT NOT NULL DEFAULT 0 CHECK(weight_remainder BETWEEN 0 AND 999999),
 PRIMARY KEY(proxy_user_id,node_id,period_id,date)
);

CREATE TABLE "xnode_imported_usage" (
 proxy_user_id BIGINT PRIMARY KEY REFERENCES xnode_identities(proxy_user_id),
 period_id TEXT REFERENCES xnode_periods(id), used_bytes BIGINT NOT NULL CHECK(used_bytes>=0),
 lifetime_bytes BIGINT NOT NULL CHECK(lifetime_bytes>=0), last_reset_at TEXT, synced_at TEXT NOT NULL
);

CREATE TABLE xnode_metered_probes (
 proxy_user_id BIGINT PRIMARY KEY REFERENCES xnode_identities(proxy_user_id),
 period_id TEXT NOT NULL REFERENCES xnode_periods(id),
 operation_id TEXT NOT NULL REFERENCES provider_operations(id),
 pair_key TEXT NOT NULL,
 endpoint_id TEXT NOT NULL,
 node_id TEXT NOT NULL REFERENCES xnode_nodes(id),
 inbound_tag TEXT NOT NULL,
 valid_until TEXT NOT NULL,
 revoked_at TEXT,
 flushed_sequence BIGINT,
 UNIQUE(operation_id,pair_key)
);

CREATE TABLE xnode_period_nodes (
 period_id TEXT NOT NULL REFERENCES xnode_periods(id), node_id TEXT NOT NULL REFERENCES xnode_nodes(id),
 flushed_sequence BIGINT, PRIMARY KEY(period_id,node_id)
);

-- Synthetic empty-installation bootstrap rows only.
INSERT INTO "affiliate_config_versions"("id","version","created_by","created_at") VALUES('affiliate-config-1',1,NULL,'2000-01-01T00:00:00Z');
INSERT INTO "affiliate_tiers"("id","config_id","position","name","success_threshold","enabled","commission_enabled","commission_bps","reward_kind","reward_coupon_id","reward_txb_minor","reward_extension_days") VALUES('affiliate-tier-1','affiliate-config-1',0,'Starter',0,1,0,0,'none',NULL,NULL,NULL);
INSERT INTO "affiliate_config_current"("singleton","config_id") VALUES(1,'affiliate-config-1');
INSERT INTO "node_compensation_config"("id","enabled","threshold_minutes","multiplier_bps","revision","created_at","updated_at") VALUES(1,0,NULL,NULL,0,'2000-01-01T00:00:00Z','2000-01-01T00:00:00Z');
INSERT INTO "onboarding_content"("kind","draft_json","published_json","draft_revision","published_revision","updated_at","published_at","updated_by") VALUES('welcome','{"en":[{"id":"hello","text":"Hi, how are you?"},{"id":"welcome","text":"Welcome to VanSail Carpool"},{"id":"setup","text":"It only takes a few seconds to complete setup."}],"zh-CN":[{"id":"hello","text":"\u4f60\u597d\uff0c\u6700\u8fd1\u600e\u4e48\u6837\uff1f"},{"id":"welcome","text":"\u6b22\u8fce\u6765\u5230 VanSail Carpool"},{"id":"setup","text":"\u53ea\u9700\u51e0\u79d2\u5373\u53ef\u5b8c\u6210\u8bbe\u7f6e\u3002"}]}','{"en":[{"id":"hello","text":"Hi, how are you?"},{"id":"welcome","text":"Welcome to VanSail Carpool"},{"id":"setup","text":"It only takes a few seconds to complete setup."}],"zh-CN":[{"id":"hello","text":"\u4f60\u597d\uff0c\u6700\u8fd1\u600e\u4e48\u6837\uff1f"},{"id":"welcome","text":"\u6b22\u8fce\u6765\u5230 VanSail Carpool"},{"id":"setup","text":"\u53ea\u9700\u51e0\u79d2\u5373\u53ef\u5b8c\u6210\u8bbe\u7f6e\u3002"}]}',1,1,'2000-01-01T00:00:00Z','2000-01-01T00:00:00Z',NULL);
INSERT INTO "onboarding_content"("kind","draft_json","published_json","draft_revision","published_revision","updated_at","published_at","updated_by") VALUES('agreements','{"en":[{"id":"private-link","icon":"link-break","title":"Keep your link private","body":"Never post, forward, sell, or share your subscription URL with another person."}],"zh-CN":[{"id":"private-link","icon":"link-break","title":"\u8bf7\u59a5\u5584\u4fdd\u7ba1\u8ba2\u9605\u94fe\u63a5","body":"\u8bf7\u52ff\u53d1\u5e03\u3001\u8f6c\u53d1\u3001\u51fa\u552e\u8ba2\u9605\u94fe\u63a5\uff0c\u6216\u4e0e\u4ed6\u4eba\u5171\u4eab\u3002"}]}','{"en":[{"id":"private-link","icon":"link-break","title":"Keep your link private","body":"Never post, forward, sell, or share your subscription URL with another person."}],"zh-CN":[{"id":"private-link","icon":"link-break","title":"\u8bf7\u59a5\u5584\u4fdd\u7ba1\u8ba2\u9605\u94fe\u63a5","body":"\u8bf7\u52ff\u53d1\u5e03\u3001\u8f6c\u53d1\u3001\u51fa\u552e\u8ba2\u9605\u94fe\u63a5\uff0c\u6216\u4e0e\u4ed6\u4eba\u5171\u4eab\u3002"}]}',1,1,'2000-01-01T00:00:00Z','2000-01-01T00:00:00Z',NULL);
INSERT INTO "payment_profiles"("id","provider","provider_name","enabled_channels","endpoint","merchant_id","credential_ciphertext","acknowledgement","enabled","created_at","updated_at") VALUES('bepusdt','bepusdt','bepusdt','','','','','',0,'2000-01-01T00:00:00Z','2000-01-01T00:00:00Z');
INSERT INTO "payment_profiles"("id","provider","provider_name","enabled_channels","endpoint","merchant_id","credential_ciphertext","acknowledgement","enabled","created_at","updated_at") VALUES('ezpay','ezpay','ezpay','','','','','',0,'2000-01-01T00:00:00Z','2000-01-01T00:00:00Z');
INSERT INTO "txb_limits"("singleton","minimum_txb_minor","maximum_txb_minor","updated_by","updated_at") VALUES(1,100,10000000000,NULL,'2000-01-01T00:00:00Z');
INSERT INTO "xnode_compile_revision"("singleton","revision") VALUES(1,2);
INSERT INTO "xnode_firewall_clock"("singleton","version","source_version","inventory_version","active_bans") VALUES(1,1,1,1,0);

INSERT INTO schema_migrations(version,applied_at) VALUES('001_initial.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('002_activity_coupons_questionnaires.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('003_emby.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('004_platform_payments_rollover_restore.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('005_database_admin_reviews.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('006_community_contract_fields.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('007_purchase_idempotency.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('008_group_message_rewards.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('009_expansion_and_cleanup.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('010_clear_subscription_cache.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('011_minimize_payment_payloads.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('012_minimize_questionnaire_imports.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('013_coupon_discards_and_courtesy_credits.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('014_goal_completion.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('015_payment_provider_profiles.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('016_multiple_payment_profiles.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('017_squad_profiles.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('018_automatic_renewal.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('019_rollover_activation_codes.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('020_release1_commerce.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('021_admin_workflows.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('022_release1_rollups.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('023_payment_callback_tombstones.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('024_operation_durability.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('025_mutation_durability.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('026_group_message_facts.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('027_host_operation_guard.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('028_connection_ip_blocks.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('029_security_and_history_indexes.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('030_affiliates.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('031_user_notifications.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('032_automatic_traffic_reset.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('033_node_compensation.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('034_abuse_qps_detector.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('035_abuse_warning_cooldown.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('036_purchase_addon_adjustments.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('037_abuse_pending_qps_index.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('038_durable_abuse_processing.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('039_abuse_record_retention_and_ip_bans.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('040_admin_user_provider_commands.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('041_admin_coupon_discard_commands.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('042_community_membership.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('043_maintenance_run_history.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('044_squad_geocheck.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('045_abuse_outbound_tag.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('046_abuse_outbound_tags.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('047_display_currency.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('048_rollover_calculated_settlement.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('049_retry_rejected_telegram_markup.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('050_restore_missing_remna_signup.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('051_lucky_draw_redesign.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('052_retry_safe_telegram_draw_jobs.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('053_retry_draw_payload_decode.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('054_restore_custom_combo_after_core_reward.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('055_restore_ordered_custom_combo_rewards.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('056_restore_custom_combo_reset_cadence.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('057_user_preferences.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('058_require_boosted_group_messages.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('059_member_squad_preferences.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('060_telegram_boost_receipts.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('061_panel_entry_verification.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('062_telegram_pm.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('063_retention_scan_indexes.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('064_telegram_pm_read_status.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('065_host_connectivity_attempts.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('066_ip_lookup.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('067_ip_lookup_compact.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('068_telegram_pm_attribution.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('069_ip_lookup_live_snapshots.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('070_ip_lookup_background.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('071_ip_lookup_current_config.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('072_builtin_usdt.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('073_browser_admin_sessions.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('073_remove_usdt_rpc_quotas.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('074_vansail_brand_and_link_auth.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('075_xnode_control_plane.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('076_xnode_final_accounting.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('077_admin_api_tokens.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('078_retire_abuse_detector.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('079_retire_legacy_controls.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('080_xnode_compile_revision.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('081_xnode_identity_routing.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('082_xnode_firewall.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('083_xnode_zero_multiplier.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('084_xnode_routing_plans.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('085_stablecoin_tokens.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('086_admin_firewall_exemptions.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('087_native_accounting_maintenance.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('088_xnode_metered_probes.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('089_xnode_member_diy.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('090_telegram_member_identity.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('091_member_whitelist.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('092_identity_rekey_immutable_references.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('093_native_only.sql',vansail_now());

INSERT INTO schema_migrations(version,applied_at) VALUES('094_delete_revoked_api_tokens.sql',vansail_now());

CREATE INDEX abuse_incident_facts_member
  ON abuse_incident_facts(user_id,created_at DESC);

CREATE INDEX abuse_records_member ON abuse_records(user_id,created_at DESC,id DESC);

CREATE INDEX activity_bets_game_created_idx ON activity_bets(game_id,created_at);

CREATE INDEX activity_bets_user_created_idx ON activity_bets(user_id, created_at DESC);

CREATE INDEX activity_checkins_user_created_idx ON activity_daily_checkins(user_id, created_at DESC);

CREATE INDEX activity_draw_results_draw_created_idx ON activity_draw_results(draw_id, created_at);

CREATE INDEX activity_draw_results_user_created_idx ON activity_draw_results(user_id, created_at DESC);

CREATE INDEX activity_extension_user_idx ON activity_extension_credits(user_id, consumed_at, created_at);

CREATE INDEX activity_group_message_events_created_idx
  ON activity_group_message_events(created_at);

CREATE INDEX activity_group_message_raw_events_created_idx
  ON activity_group_message_raw_events(created_at);

CREATE INDEX activity_group_message_windows_date_idx
  ON activity_group_message_windows(local_date, updated_at);

CREATE INDEX activity_prizes_draw_idx ON activity_lucky_prizes(draw_id, position);

CREATE UNIQUE INDEX activity_raffle_open_command_idx ON activity_lucky_draws(lower(command))
  WHERE kind='raffle' AND status IN ('publishing','open') AND command<>'';

CREATE UNIQUE INDEX activity_raffle_open_keyword_idx ON activity_lucky_draws(lower(keyword))
  WHERE kind='raffle' AND status IN ('publishing','open') AND keyword<>'';

CREATE INDEX activity_raffle_tickets_draw_status_idx ON activity_raffle_tickets(draw_id,status,created_at,id);

CREATE INDEX admin_api_tokens_owner ON admin_api_tokens(user_id,created_at DESC);

CREATE INDEX admin_temporary_bans_due ON admin_temporary_bans(expires_at) WHERE restored_at IS NULL;

CREATE INDEX affiliate_settlements_inviter_idx ON affiliate_settlements(inviter_user_id,settled_at DESC,id DESC);

CREATE UNIQUE INDEX affiliate_tiers_enabled_threshold_idx ON affiliate_tiers(config_id,success_threshold) WHERE enabled=1;

CREATE INDEX audit_created_idx ON audit_events(created_at DESC);

CREATE UNIQUE INDEX backup_runs_upload_idempotency_idx
  ON backup_runs(actor_user_id,idempotency_key)
  WHERE source='upload' AND actor_user_id IS NOT NULL AND idempotency_key<>'';

CREATE INDEX backup_uploads_status_updated_idx ON backup_uploads(status,updated_at,id);

CREATE INDEX browser_access_links_expiry_idx ON browser_access_links(expires_at);

CREATE UNIQUE INDEX connection_ip_blocks_expiry_job_idx
  ON connection_ip_blocks(expiry_job_id);

CREATE INDEX connection_ip_blocks_owner_expiry_idx
  ON connection_ip_blocks(user_id,expires_at,id);

CREATE INDEX connection_scans_owner_expiry_idx
  ON connection_scans(user_id,expires_at DESC,id);

CREATE INDEX coupon_active_expiry_idx ON coupon_definitions(active, expires_at);

CREATE INDEX coupon_grant_discards_user_idx ON coupon_grant_discards(user_id, discarded_at DESC);

CREATE INDEX coupon_grants_user_status_idx ON coupon_grants(user_id, status, created_at DESC);

CREATE INDEX coupon_redemptions_coupon_user_idx ON coupon_redemptions(coupon_id, user_id, created_at);

CREATE INDEX coupon_uses_coupon_user_idx ON coupon_uses(coupon_id, user_id, created_at);

CREATE INDEX courtesy_credits_created_idx ON courtesy_credits(created_at DESC);

CREATE INDEX database_admin_reviews_expiry_idx
  ON database_admin_reviews(expires_at, consumed_at);

CREATE INDEX emby_accounts_status_idx ON emby_accounts(status,updated_at);

CREATE UNIQUE INDEX emby_setup_open_operation_idx
  ON provider_operations(owner_user_id,kind)
  WHERE kind='emby_setup' AND status IN ('queued','processing','pending_review');

CREATE INDEX firewall_bans_detecting_ip_active ON xnode_firewall_bans(ip)
 WHERE released_version=0;

CREATE INDEX host_connectivity_attempts_host_idx ON host_connectivity_attempts(host_uuid,started_at DESC,id DESC);

CREATE INDEX host_connectivity_attempts_latest_idx ON host_connectivity_attempts(config_hash,host_uuid,started_at DESC,id DESC);

CREATE INDEX host_connectivity_attempts_started_idx ON host_connectivity_attempts(started_at DESC,id DESC);

CREATE INDEX ip_lookup_cache ON ip_lookup_reports(ip,julianday(completed_at) DESC)
WHERE status IN ('succeeded','partial');

CREATE INDEX ip_lookup_cache_key ON ip_lookup_reports(cache_key,status);

CREATE INDEX ip_lookup_check_report ON ip_lookup_checks(report_id);

CREATE UNIQUE INDEX ip_lookup_inflight ON ip_lookup_reports(ip) WHERE status='processing';

CREATE INDEX ip_lookup_live_cache ON ip_lookup_live_snapshots(ip,completed_at DESC);

CREATE INDEX ledger_user_created_idx ON ledger_entries(user_id, created_at DESC);

CREATE INDEX maintenance_runs_local_date_idx
  ON maintenance_runs(local_date,started_at DESC,id DESC);

CREATE INDEX maintenance_runs_status_lease_idx
  ON maintenance_runs(status,lease_expires_at);

CREATE INDEX node_compensation_events_created_idx ON node_compensation_events(created_at);

CREATE INDEX node_compensation_events_status_cursor
  ON node_compensation_events(status,created_at DESC,id DESC);

CREATE UNIQUE INDEX node_compensation_one_observing_per_node
  ON node_compensation_events(node_uuid) WHERE status='observing';

CREATE UNIQUE INDEX node_compensation_review_keys
  ON node_compensation_events(reviewed_by,review_idempotency_key)
  WHERE reviewed_by IS NOT NULL AND review_idempotency_key IS NOT NULL;

CREATE UNIQUE INDEX outbox_active_payload_idx ON outbox_jobs(kind,payload)
  WHERE status IN ('pending','processing');

CREATE INDEX outbox_available_idx ON outbox_jobs(status,available_at);

CREATE INDEX outbox_failed_retention_idx ON outbox_jobs(updated_at,id) WHERE status='failed';

CREATE INDEX outbox_jobs_admin_page_idx ON outbox_jobs(created_at DESC,id DESC);

CREATE INDEX outbox_jobs_admin_status_page_idx ON outbox_jobs(status,created_at DESC,id DESC);

CREATE INDEX payment_callback_tombstones_order_idx
  ON payment_callback_tombstones(order_id,provider);

CREATE INDEX payment_orders_admin_page_idx ON payment_orders(created_at DESC,id DESC);

CREATE INDEX payment_orders_admin_status_page_idx ON payment_orders(status,created_at DESC,id DESC);

CREATE INDEX payment_orders_method_created_idx ON payment_orders(method_id,created_at DESC);

CREATE INDEX payment_orders_user_created_idx ON payment_orders(user_id,created_at DESC);

CREATE INDEX payment_profiles_provider_idx ON payment_profiles(provider, enabled, id);

CREATE INDEX provider_operation_items_status_idx
  ON provider_operation_items(operation_id,status,item_key);

CREATE UNIQUE INDEX provider_operation_replays_resolution_key_idx
  ON provider_operation_replays(operation_id,actor_user_id,idempotency_key);

CREATE INDEX provider_operations_owner_created_idx
  ON provider_operations(owner_user_id,created_at DESC,id DESC);

CREATE INDEX provider_operations_status_updated_idx
  ON provider_operations(status,updated_at,id);

CREATE INDEX purchase_addon_adjustments_purchase_created_idx
  ON purchase_addon_adjustments(purchase_id,created_at DESC,id);

CREATE INDEX purchase_addons_squad_idx ON purchase_addons(group_id,purchase_id);

CREATE INDEX purchase_history_tombstones_combo_created_idx
  ON purchase_history_tombstones(combo_id,created_at DESC,purchase_id);

CREATE INDEX purchase_history_tombstones_user_created_idx
  ON purchase_history_tombstones(user_id,created_at DESC,purchase_id);

CREATE INDEX purchase_rollovers_status_idx ON purchase_rollovers(status,updated_at);

CREATE INDEX purchases_admin_page_idx ON purchases(created_at DESC,id DESC);

CREATE INDEX purchases_admin_status_page_idx ON purchases(status,created_at DESC,id DESC);

CREATE INDEX purchases_auto_renew_due_idx
  ON purchases(auto_renew_enabled,status,valid_until);

CREATE UNIQUE INDEX purchases_auto_renew_successor_idx
  ON purchases(auto_renew_source_purchase_id)
  WHERE auto_renew_source_purchase_id IS NOT NULL;

CREATE INDEX purchases_combo_created_idx ON purchases(combo_id,created_at);

CREATE UNIQUE INDEX purchases_renewal_term_idx ON purchases(renewal_batch_id, renewal_index)
  WHERE renewal_batch_id IS NOT NULL;

CREATE UNIQUE INDEX purchases_user_idempotency_idx
  ON purchases(user_id,idempotency_key)
  WHERE idempotency_key IS NOT NULL;

CREATE INDEX purchases_user_status_idx ON purchases(user_id, status, valid_until);

CREATE INDEX purchases_xnode_from_idx ON purchases(valid_from);

CREATE INDEX purchases_xnode_until_idx ON purchases(valid_until);

CREATE INDEX questionnaire_imports_status_idx ON questionnaire_imports(status, created_at);

CREATE INDEX questionnaire_participants_lookup_idx ON questionnaire_participants(questionnaire_id, validation_code);

CREATE INDEX questionnaire_participants_questionnaire_awarded_idx
  ON questionnaire_participants(questionnaire_id,awarded_at);

CREATE INDEX questionnaire_participants_user_history_idx
  ON questionnaire_participants(user_id,created_at DESC,id DESC);

CREATE INDEX questionnaires_active_closes_idx ON questionnaires(status, closes_at);

CREATE INDEX questionnaires_created_idx ON questionnaires(created_at DESC);

CREATE UNIQUE INDEX questionnaires_single_active_idx ON questionnaires((1)) WHERE status='active';

CREATE INDEX refunds_admin_status_page_idx ON refunds(status,created_at DESC,id DESC);

CREATE INDEX refunds_created_idx ON refunds(created_at DESC);

CREATE INDEX restore_jobs_created_idx ON restore_jobs(created_at DESC);

CREATE UNIQUE INDEX restore_jobs_request_key_idx
  ON restore_jobs(request_actor_id,idempotency_key)
  WHERE idempotency_key <> '';

CREATE INDEX rollover_member_daily_user_date_idx
  ON rollover_member_daily_rollups(user_id,local_date);

CREATE UNIQUE INDEX sessions_miniapp_user_unique_idx ON sessions(user_id) WHERE scope='miniapp';

CREATE INDEX sessions_scope_expiry_idx ON sessions(scope,expires_at);

CREATE INDEX sessions_user_id_idx ON sessions(user_id);

CREATE INDEX telegram_pm_conversation_page_idx ON telegram_pm_conversations(created_at DESC,id DESC);

CREATE INDEX telegram_pm_payload_expiry_idx ON telegram_pm_payloads(expires_at);

CREATE UNIQUE INDEX telegram_pm_profile_attempt_idx ON provider_operation_items(
    CASE WHEN target_type='pm_topic_recovery' THEN substr(target_id,1,instr(target_id,':')-1) ELSE target_id END
) WHERE status='processing' AND ((target_type='pm_conversation' AND item_key='profile') OR target_type='pm_topic_recovery');

CREATE UNIQUE INDEX telegram_pm_topic_idx ON telegram_pm_conversations(chat_id,topic_id) WHERE topic_id IS NOT NULL;

CREATE INDEX telegram_pm_update_retention_idx ON telegram_pm_updates(created_at);

CREATE INDEX usdt_transfers_match_idx ON usdt_transfers(network,token,recipient,status);

CREATE INDEX usdt_transfers_review_idx ON usdt_transfers(status,received_at,id);

CREATE INDEX user_notification_events_gate_idx
  ON user_notification_events(gate_key,queued_at)
  WHERE queued_at IS NULL;

CREATE INDEX user_notification_events_source_idx
  ON user_notification_events(source_kind,source_id);

CREATE INDEX users_admin_page_idx ON users(created_at DESC,id DESC);

CREATE INDEX users_inviter_created_idx ON users(inviter_id,created_at DESC);

CREATE INDEX xnode_daily_usage_date_node_idx ON xnode_daily_usage(date,node_id);

CREATE INDEX xnode_daily_usage_user_date_idx ON xnode_daily_usage(proxy_user_id,date,node_id,period_id);

CREATE INDEX xnode_endpoints_node_idx ON xnode_endpoints(node_id,inbound_tag);

CREATE UNIQUE INDEX xnode_firewall_active_prefix ON xnode_firewall_bans(prefix) WHERE released_version=0;

CREATE UNIQUE INDEX xnode_firewall_active_source ON xnode_firewall_sources(id) WHERE released_version=0;

CREATE INDEX xnode_firewall_ban_versions ON xnode_firewall_bans(issued_version,released_version);

CREATE INDEX xnode_firewall_exemption_versions ON xnode_firewall_exemptions(issued_version,released_version);

CREATE INDEX xnode_legacy_history_date_node_idx ON "xnode_imported_history"(date,node_id);

CREATE INDEX xnode_metered_probes_period ON xnode_metered_probes(period_id,node_id);

CREATE INDEX xnode_periods_purchase_idx ON xnode_periods(purchase_id);

CREATE INDEX xnode_periods_user_idx ON xnode_periods(proxy_user_id,started_at);

CREATE INDEX xnode_service_bindings_expiry_idx ON xnode_service_bindings(valid_until);

-- +goose StatementBegin
CREATE FUNCTION "affiliate_config_versions_no_delete_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
RAISE EXCEPTION USING MESSAGE='affiliate config history is immutable', ERRCODE='23514';
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "affiliate_config_versions_no_delete" BEFORE DELETE ON "affiliate_config_versions" FOR EACH ROW EXECUTE FUNCTION "affiliate_config_versions_no_delete_fn"();

-- +goose StatementBegin
CREATE FUNCTION "affiliate_config_versions_no_update_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
IF NOT (
  OLD.id IS NOT DISTINCT FROM NEW.id
  AND OLD.version IS NOT DISTINCT FROM NEW.version
  AND OLD.created_at IS NOT DISTINCT FROM NEW.created_at
  AND EXISTS (
    SELECT 1 FROM member_identity_aliases a
    WHERE (OLD.created_by=a.old_user_id
           AND NEW.created_by IN (a.user_id,'~tg:'||a.user_id))
       OR (OLD.created_by='~tg:'||a.user_id AND NEW.created_by=a.user_id)
  )
) THEN
RAISE EXCEPTION USING MESSAGE='affiliate config history is immutable', ERRCODE='23514';
END IF;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "affiliate_config_versions_no_update" BEFORE UPDATE ON "affiliate_config_versions" FOR EACH ROW EXECUTE FUNCTION "affiliate_config_versions_no_update_fn"();

-- +goose StatementBegin
CREATE FUNCTION "affiliate_tiers_no_delete_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
RAISE EXCEPTION USING MESSAGE='affiliate tier history is immutable', ERRCODE='23514';
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "affiliate_tiers_no_delete" BEFORE DELETE ON "affiliate_tiers" FOR EACH ROW EXECUTE FUNCTION "affiliate_tiers_no_delete_fn"();

-- +goose StatementBegin
CREATE FUNCTION "affiliate_tiers_no_update_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
RAISE EXCEPTION USING MESSAGE='affiliate tier history is immutable', ERRCODE='23514';
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "affiliate_tiers_no_update" BEFORE UPDATE ON "affiliate_tiers" FOR EACH ROW EXECUTE FUNCTION "affiliate_tiers_no_update_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_admin_temporary_bans_delete_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_admin_temporary_bans_delete" AFTER DELETE ON "admin_temporary_bans" FOR EACH ROW EXECUTE FUNCTION "compile_admin_temporary_bans_delete_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_admin_temporary_bans_insert_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_admin_temporary_bans_insert" AFTER INSERT ON "admin_temporary_bans" FOR EACH ROW EXECUTE FUNCTION "compile_admin_temporary_bans_insert_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_admin_temporary_bans_update_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_admin_temporary_bans_update" AFTER UPDATE ON "admin_temporary_bans" FOR EACH ROW EXECUTE FUNCTION "compile_admin_temporary_bans_update_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_combo_whitelist_update_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_combo_whitelist_update" AFTER UPDATE OF whitelist_ip_quota ON "combos" FOR EACH ROW EXECUTE FUNCTION "compile_combo_whitelist_update_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_purchases_delete_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_purchases_delete" AFTER DELETE ON "purchases" FOR EACH ROW EXECUTE FUNCTION "compile_purchases_delete_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_purchases_insert_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_purchases_insert" AFTER INSERT ON "purchases" FOR EACH ROW EXECUTE FUNCTION "compile_purchases_insert_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_purchases_update_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_purchases_update" AFTER UPDATE ON "purchases" FOR EACH ROW EXECUTE FUNCTION "compile_purchases_update_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_routing_templates_insert_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
IF NEW.key='xnode.routing_templates' THEN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
END IF;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_routing_templates_insert" AFTER INSERT ON "settings" FOR EACH ROW EXECUTE FUNCTION "compile_routing_templates_insert_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_routing_templates_update_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
IF NEW.key='xnode.routing_templates' THEN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
END IF;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_routing_templates_update" AFTER UPDATE ON "settings" FOR EACH ROW EXECUTE FUNCTION "compile_routing_templates_update_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_settings_delete_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
IF OLD.key='xnode.global' THEN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
END IF;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_settings_delete" AFTER DELETE ON "settings" FOR EACH ROW EXECUTE FUNCTION "compile_settings_delete_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_settings_insert_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
IF NEW.key='xnode.global' THEN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
END IF;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_settings_insert" AFTER INSERT ON "settings" FOR EACH ROW EXECUTE FUNCTION "compile_settings_insert_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_settings_update_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
IF NEW.key='xnode.global' THEN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
END IF;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_settings_update" AFTER UPDATE ON "settings" FOR EACH ROW EXECUTE FUNCTION "compile_settings_update_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_user_disabled_squads_delete_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_user_disabled_squads_delete" AFTER DELETE ON "user_disabled_squads" FOR EACH ROW EXECUTE FUNCTION "compile_user_disabled_squads_delete_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_user_disabled_squads_insert_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_user_disabled_squads_insert" AFTER INSERT ON "user_disabled_squads" FOR EACH ROW EXECUTE FUNCTION "compile_user_disabled_squads_insert_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_user_disabled_squads_update_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_user_disabled_squads_update" AFTER UPDATE ON "user_disabled_squads" FOR EACH ROW EXECUTE FUNCTION "compile_user_disabled_squads_update_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_users_delete_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_users_delete" AFTER DELETE ON "users" FOR EACH ROW EXECUTE FUNCTION "compile_users_delete_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_users_insert_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_users_insert" AFTER INSERT ON "users" FOR EACH ROW EXECUTE FUNCTION "compile_users_insert_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_users_update_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
IF OLD.onboarding_state IS DISTINCT FROM  NEW.onboarding_state THEN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
END IF;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_users_update" AFTER UPDATE OF onboarding_state ON "users" FOR EACH ROW EXECUTE FUNCTION "compile_users_update_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_config_snippets_delete_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_config_snippets_delete" AFTER DELETE ON "xnode_config_snippets" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_config_snippets_delete_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_config_snippets_insert_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_config_snippets_insert" AFTER INSERT ON "xnode_config_snippets" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_config_snippets_insert_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_config_snippets_update_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_config_snippets_update" AFTER UPDATE ON "xnode_config_snippets" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_config_snippets_update_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_configs_delete_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_configs_delete" AFTER DELETE ON "xnode_configs" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_configs_delete_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_configs_insert_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_configs_insert" AFTER INSERT ON "xnode_configs" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_configs_insert_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_configs_update_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_configs_update" AFTER UPDATE ON "xnode_configs" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_configs_update_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_customizations_delete_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_customizations_delete" AFTER DELETE ON "xnode_customizations" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_customizations_delete_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_customizations_insert_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_customizations_insert" AFTER INSERT ON "xnode_customizations" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_customizations_insert_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_customizations_update_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_customizations_update" AFTER UPDATE ON "xnode_customizations" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_customizations_update_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_group_bindings_delete_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_group_bindings_delete" AFTER DELETE ON "xnode_group_bindings" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_group_bindings_delete_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_group_bindings_insert_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_group_bindings_insert" AFTER INSERT ON "xnode_group_bindings" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_group_bindings_insert_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_group_bindings_update_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_group_bindings_update" AFTER UPDATE ON "xnode_group_bindings" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_group_bindings_update_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_groups_delete_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_groups_delete" AFTER DELETE ON "xnode_groups" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_groups_delete_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_groups_insert_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_groups_insert" AFTER INSERT ON "xnode_groups" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_groups_insert_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_groups_update_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_groups_update" AFTER UPDATE ON "xnode_groups" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_groups_update_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_identities_delete_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_identities_delete" AFTER DELETE ON "xnode_identities" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_identities_delete_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_identities_insert_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_identities_insert" AFTER INSERT ON "xnode_identities" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_identities_insert_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_identities_update_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
IF OLD.credentials_cipher IS DISTINCT FROM  NEW.credentials_cipher OR OLD.current_period_id IS DISTINCT FROM  NEW.current_period_id OR OLD.access_disabled IS DISTINCT FROM  NEW.access_disabled THEN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
END IF;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_identities_update" AFTER UPDATE OF credentials_cipher,current_period_id,access_disabled ON "xnode_identities" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_identities_update_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_nodes_delete_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_nodes_delete" AFTER DELETE ON "xnode_nodes" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_nodes_delete_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_nodes_insert_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_nodes_insert" AFTER INSERT ON "xnode_nodes" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_nodes_insert_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_nodes_update_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
IF OLD.config_id IS DISTINCT FROM  NEW.config_id OR OLD.settings_cipher IS DISTINCT FROM  NEW.settings_cipher OR OLD.enabled IS DISTINCT FROM  NEW.enabled THEN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
END IF;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_nodes_update" AFTER UPDATE OF config_id,settings_cipher,enabled ON "xnode_nodes" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_nodes_update_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_periods_delete_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_periods_delete" AFTER DELETE ON "xnode_periods" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_periods_delete_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_periods_insert_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_periods_insert" AFTER INSERT ON "xnode_periods" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_periods_insert_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_periods_update_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
IF OLD.status IS DISTINCT FROM  NEW.status OR OLD.limit_bytes IS DISTINCT FROM  NEW.limit_bytes OR OLD.strategy IS DISTINCT FROM  NEW.strategy OR (OLD.limit_bytes>0 AND OLD.used_bytes>=OLD.limit_bytes) IS DISTINCT FROM  (NEW.limit_bytes>0 AND NEW.used_bytes>=NEW.limit_bytes) THEN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
END IF;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_periods_update" AFTER UPDATE OF status,limit_bytes,strategy,used_bytes ON "xnode_periods" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_periods_update_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_probe_insert_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_probe_insert" AFTER INSERT ON "xnode_metered_probes" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_probe_insert_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_probe_revoke_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_probe_revoke" AFTER UPDATE OF revoked_at ON "xnode_metered_probes" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_probe_revoke_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_routing_insert_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_routing_insert" AFTER INSERT ON "xnode_routing_plans" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_routing_insert_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_routing_update_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_routing_update" AFTER UPDATE ON "xnode_routing_plans" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_routing_update_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_service_bindings_delete_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_service_bindings_delete" AFTER DELETE ON "xnode_service_bindings" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_service_bindings_delete_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_service_bindings_insert_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_service_bindings_insert" AFTER INSERT ON "xnode_service_bindings" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_service_bindings_insert_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_service_bindings_update_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_service_bindings_update" AFTER UPDATE ON "xnode_service_bindings" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_service_bindings_update_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_snippets_delete_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_snippets_delete" AFTER DELETE ON "xnode_snippets" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_snippets_delete_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_snippets_insert_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_snippets_insert" AFTER INSERT ON "xnode_snippets" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_snippets_insert_fn"();

-- +goose StatementBegin
CREATE FUNCTION "compile_xnode_snippets_update_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "compile_xnode_snippets_update" AFTER UPDATE ON "xnode_snippets" FOR EACH ROW EXECUTE FUNCTION "compile_xnode_snippets_update_fn"();

-- +goose StatementBegin
CREATE FUNCTION "firewall_exemption_inventory_insert_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_firewall_clock SET inventory_version=version WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "firewall_exemption_inventory_insert" AFTER INSERT ON "xnode_firewall_exemptions" FOR EACH ROW EXECUTE FUNCTION "firewall_exemption_inventory_insert_fn"();

-- +goose StatementBegin
CREATE FUNCTION "firewall_exemption_inventory_release_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
IF OLD.released_version=0 AND NEW.released_version>0 THEN
UPDATE xnode_firewall_clock SET inventory_version=version WHERE singleton=1;
END IF;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "firewall_exemption_inventory_release" AFTER UPDATE OF released_version ON "xnode_firewall_exemptions" FOR EACH ROW EXECUTE FUNCTION "firewall_exemption_inventory_release_fn"();

-- +goose StatementBegin
CREATE FUNCTION "firewall_inventory_insert_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
IF NEW.released_version=0 THEN
UPDATE xnode_firewall_clock SET inventory_version=version,active_bans=active_bans+1 WHERE singleton=1;
END IF;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "firewall_inventory_insert" AFTER INSERT ON "xnode_firewall_bans" FOR EACH ROW EXECUTE FUNCTION "firewall_inventory_insert_fn"();

-- +goose StatementBegin
CREATE FUNCTION "firewall_inventory_release_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
IF OLD.released_version=0 AND NEW.released_version>0 THEN
UPDATE xnode_firewall_clock SET inventory_version=version,active_bans=active_bans-1 WHERE singleton=1;
END IF;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "firewall_inventory_release" AFTER UPDATE OF released_version ON "xnode_firewall_bans" FOR EACH ROW EXECUTE FUNCTION "firewall_inventory_release_fn"();

-- +goose StatementBegin
CREATE FUNCTION "preference_visibility_after_transition_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE user_preferences SET show_around_tx=0,show_activity=0
  WHERE user_id=NEW.user_id AND NOT EXISTS (
    SELECT 1 FROM purchases WHERE user_id=NEW.user_id AND status='active'
      AND julianday(valid_from)<=julianday(NEW.updated_at)
      AND julianday(valid_until)>julianday(NEW.updated_at)
  );
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "preference_visibility_after_transition" AFTER UPDATE OF status,valid_from,valid_until ON "purchases" FOR EACH ROW EXECUTE FUNCTION "preference_visibility_after_transition_fn"();

-- +goose StatementBegin
CREATE FUNCTION "preference_visibility_before_purchase_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE user_preferences SET show_around_tx=0,show_activity=0
  WHERE user_id=NEW.user_id AND NOT EXISTS (
    SELECT 1 FROM purchases WHERE user_id=NEW.user_id AND status='active'
      AND julianday(valid_from)<=julianday(NEW.created_at)
      AND julianday(valid_until)>julianday(NEW.created_at)
  );
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "preference_visibility_before_purchase" BEFORE INSERT ON "purchases" FOR EACH ROW EXECUTE FUNCTION "preference_visibility_before_purchase_fn"();

-- +goose StatementBegin
CREATE FUNCTION "preference_visibility_before_transition_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE user_preferences SET show_around_tx=0,show_activity=0
  WHERE user_id=NEW.user_id AND NOT EXISTS (
    SELECT 1 FROM purchases WHERE user_id=NEW.user_id AND status='active'
      AND julianday(valid_from)<=julianday(NEW.updated_at)
      AND julianday(valid_until)>julianday(NEW.updated_at)
  );
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "preference_visibility_before_transition" BEFORE UPDATE OF status,valid_from,valid_until ON "purchases" FOR EACH ROW EXECUTE FUNCTION "preference_visibility_before_transition_fn"();

-- +goose StatementBegin
CREATE FUNCTION "provider_operation_items_open_host_guard_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
IF NEW.target_type='remnawave_host'
  AND EXISTS (
    SELECT 1 FROM provider_operations incoming
    WHERE incoming.id=NEW.operation_id AND incoming.kind='host_remark_update'
  )
  AND EXISTS (
    SELECT 1 FROM provider_operation_items existing_item
    JOIN provider_operations existing_operation
      ON existing_operation.id=existing_item.operation_id
    WHERE existing_item.target_type='remnawave_host'
      AND existing_item.target_id=NEW.target_id
      AND existing_operation.kind='host_remark_update'
      AND existing_operation.status IN ('queued','processing','pending_review','partial')
      AND existing_operation.id<>NEW.operation_id
  ) THEN
RAISE EXCEPTION USING MESSAGE='open host remark operation exists', ERRCODE='23514';
END IF;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "provider_operation_items_open_host_guard" BEFORE INSERT ON "provider_operation_items" FOR EACH ROW EXECUTE FUNCTION "provider_operation_items_open_host_guard_fn"();

-- +goose StatementBegin
CREATE FUNCTION "retire_xnode_firewall_member_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_firewall_members SET active=0 WHERE node_id=OLD.id;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "retire_xnode_firewall_member" AFTER DELETE ON "xnode_nodes" FOR EACH ROW EXECUTE FUNCTION "retire_xnode_firewall_member_fn"();

-- +goose StatementBegin
CREATE FUNCTION "usdt_terminal_evidence_immutable_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
IF OLD.status IN ('credited','dismissed') THEN
RAISE EXCEPTION USING MESSAGE='terminal transfer evidence is immutable', ERRCODE='23514';
END IF;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "usdt_terminal_evidence_immutable" BEFORE UPDATE ON "usdt_transfers" FOR EACH ROW EXECUTE FUNCTION "usdt_terminal_evidence_immutable_fn"();

-- +goose StatementBegin
CREATE FUNCTION "user_notification_events_immutable_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
IF NOT (
  OLD.source_kind IS NOT DISTINCT FROM NEW.source_kind
  AND OLD.source_id IS NOT DISTINCT FROM NEW.source_id
  AND OLD.kind IS NOT DISTINCT FROM NEW.kind
  AND OLD.payload_json IS NOT DISTINCT FROM NEW.payload_json
  AND OLD.gate_key IS NOT DISTINCT FROM NEW.gate_key
  AND OLD.created_at IS NOT DISTINCT FROM NEW.created_at
  AND EXISTS (
    SELECT 1 FROM member_identity_aliases a
    WHERE (OLD.user_id=a.old_user_id
           AND NEW.user_id IN (a.user_id,'~tg:'||a.user_id))
       OR (OLD.user_id='~tg:'||a.user_id AND NEW.user_id=a.user_id)
  )
) THEN
RAISE EXCEPTION USING MESSAGE='user notification payloads are immutable', ERRCODE='23514';
END IF;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER "user_notification_events_immutable" BEFORE UPDATE OF user_id,source_kind,source_id,kind,payload_json,gate_key,created_at ON "user_notification_events" FOR EACH ROW EXECUTE FUNCTION "user_notification_events_immutable_fn"();

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative database downgrades require a separately verified recovery'; END $$;
-- +goose StatementEnd
