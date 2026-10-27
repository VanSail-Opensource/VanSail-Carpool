-- Generated from database.Open plus completed Telegram identity migration.
-- Historical SQLite migration files remain immutable.
CREATE TABLE abuse_incident_facts (
  incident_id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  incident_bucket_at TEXT NOT NULL,
  selected_action TEXT NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE(user_id,incident_bucket_at)
);

CREATE TABLE abuse_notification_deliveries (
  record_id TEXT NOT NULL REFERENCES abuse_records(id) ON DELETE CASCADE, recipient_telegram_id INTEGER NOT NULL,
  kind TEXT NOT NULL, delivered_at TEXT, created_at TEXT NOT NULL, PRIMARY KEY(record_id,recipient_telegram_id,kind)
);

CREATE TABLE abuse_record_nodes (record_id TEXT NOT NULL REFERENCES abuse_records(id) ON DELETE CASCADE, node_uuid TEXT NOT NULL, PRIMARY KEY(record_id,node_uuid));

CREATE TABLE abuse_record_reasons (record_id TEXT NOT NULL REFERENCES abuse_records(id) ON DELETE CASCADE, name TEXT NOT NULL, PRIMARY KEY(record_id,name));

CREATE TABLE abuse_records (
  id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, incident_bucket_at TEXT NOT NULL,
  measured_qps INTEGER NOT NULL CHECK(measured_qps >= 0), qps_limit INTEGER NOT NULL CHECK(qps_limit >= 1), selected_action TEXT NOT NULL,
  expires_at TEXT, deleted_at TEXT, created_at TEXT NOT NULL, punishment_completed_at TEXT, UNIQUE(user_id,incident_bucket_at)
);

CREATE TABLE abuse_temp_bans (
  record_id TEXT PRIMARY KEY REFERENCES abuse_records(id) ON DELETE CASCADE, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at TEXT NOT NULL, restore_queued_at TEXT, restored_at TEXT, created_at TEXT NOT NULL
);

CREATE TABLE activity_bets (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  game_id TEXT NOT NULL REFERENCES activity_games(id),
  stake_minor INTEGER NOT NULL CHECK (stake_minor > 0),
  won INTEGER NOT NULL,
  payout_minor INTEGER NOT NULL CHECK (payout_minor >= 0),
  balance_after_minor INTEGER NOT NULL CHECK (balance_after_minor >= 0),
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
  reward_minor INTEGER NOT NULL CHECK (reward_minor >= 0),
  balance_after_minor INTEGER NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE(user_id, local_date)
);

CREATE TABLE activity_daily_rollups (
  local_date TEXT PRIMARY KEY,
  checkin_count INTEGER NOT NULL DEFAULT 0 CHECK (checkin_count >= 0),
  checkin_reward_txb_minor INTEGER NOT NULL DEFAULT 0 CHECK (checkin_reward_txb_minor >= 0),
  group_message_count INTEGER NOT NULL DEFAULT 0 CHECK (group_message_count >= 0),
  group_message_reward_txb_minor INTEGER NOT NULL DEFAULT 0 CHECK (group_message_reward_txb_minor >= 0),
  updated_at TEXT NOT NULL
);

CREATE TABLE activity_draw_delivery (
  delivery_key TEXT PRIMARY KEY,
  chat_id INTEGER NOT NULL,
  message_id INTEGER NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE "activity_draw_results" (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  draw_id TEXT NOT NULL,
  prize_id TEXT NOT NULL,
  prize_name TEXT NOT NULL,
  fee_minor INTEGER NOT NULL CHECK (fee_minor >= 0),
  reward_kind TEXT NOT NULL,
  reward_payload TEXT NOT NULL,
  balance_after_minor INTEGER NOT NULL CHECK (balance_after_minor >= 0),
  configuration_snapshot TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE(user_id, idempotency_key)
);

CREATE TABLE activity_draw_revisions (
  id TEXT PRIMARY KEY,
  draw_id TEXT NOT NULL REFERENCES activity_lucky_draws(id) ON DELETE CASCADE,
  revision INTEGER NOT NULL,
  snapshot_json TEXT NOT NULL CHECK (json_valid(snapshot_json)),
  created_at TEXT NOT NULL,
  UNIQUE(draw_id,revision)
);

CREATE TABLE activity_extension_credits (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  days INTEGER NOT NULL CHECK (days BETWEEN 1 AND 3650),
  source_type TEXT NOT NULL,
  source_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  consumed_at TEXT,
  consumed_by_purchase_id TEXT REFERENCES purchases(id), hours INTEGER CHECK (hours IS NULL OR hours > 0),
  UNIQUE(source_type, source_id)
);

CREATE TABLE activity_games (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  icon TEXT NOT NULL DEFAULT '',
  enabled INTEGER NOT NULL DEFAULT 0,
  win_chance_bps INTEGER NOT NULL CHECK (win_chance_bps BETWEEN 0 AND 10000),
  minimum_stake_minor INTEGER NOT NULL CHECK (minimum_stake_minor > 0),
  maximum_stake_minor INTEGER NOT NULL CHECK (maximum_stake_minor >= minimum_stake_minor),
  return_multiplier_bps INTEGER NOT NULL CHECK (return_multiplier_bps BETWEEN 10000 AND 1000000),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
, description TEXT NOT NULL DEFAULT '');

CREATE TABLE activity_group_message_events (
  chat_id INTEGER NOT NULL,
  message_id INTEGER NOT NULL,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  local_date TEXT NOT NULL,
  counted INTEGER NOT NULL DEFAULT 0 CHECK (counted IN (0,1)),
  created_at TEXT NOT NULL,
  PRIMARY KEY (chat_id, message_id)
);

CREATE TABLE activity_group_message_raw_events (
  chat_id INTEGER NOT NULL,
  message_id INTEGER NOT NULL,
  local_date TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY (chat_id,message_id)
);

CREATE TABLE activity_group_message_windows (
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  local_date TEXT NOT NULL,
  message_count INTEGER NOT NULL DEFAULT 0 CHECK (message_count >= 0),
  rewarded_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (user_id, local_date)
);

CREATE TABLE activity_lucky_draws (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  kind TEXT NOT NULL CHECK (kind IN ('instant','raffle')),
  status TEXT NOT NULL CHECK (status IN ('draft','publishing','open','settling','completed','cancelled')),
  fee_minor INTEGER NOT NULL CHECK (fee_minor > 0),
  expected_participation INTEGER CHECK (expected_participation IS NULL OR expected_participation > 0),
  threshold INTEGER CHECK (threshold IS NULL OR threshold > 0),
  keyword TEXT NOT NULL DEFAULT '',
  command TEXT NOT NULL DEFAULT '',
  group_chat_id INTEGER,
  announcement_message_id INTEGER,
  revision INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  CHECK ((kind='instant' AND expected_participation IS NOT NULL AND threshold IS NULL)
      OR (kind='raffle' AND threshold IS NOT NULL AND expected_participation IS NULL))
);

CREATE TABLE activity_lucky_prizes (
  id TEXT PRIMARY KEY,
  draw_id TEXT NOT NULL REFERENCES activity_lucky_draws(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  position INTEGER NOT NULL CHECK (position >= 0),
  probability_bps INTEGER CHECK (probability_bps IS NULL OR probability_bps BETWEEN 1 AND 10000),
  stock INTEGER CHECK (stock IS NULL OR stock > 0),
  reward_payload TEXT NOT NULL CHECK (json_valid(reward_payload)),
  UNIQUE(draw_id, position),
  CHECK ((probability_bps IS NOT NULL AND stock IS NULL) OR (probability_bps IS NULL AND stock IS NOT NULL))
);

CREATE TABLE activity_raffle_tickets (
  id TEXT PRIMARY KEY,
  draw_id TEXT NOT NULL REFERENCES activity_lucky_draws(id),
  user_id TEXT NOT NULL REFERENCES users(id),
  chat_id INTEGER NOT NULL,
  message_id INTEGER NOT NULL,
  fee_minor INTEGER NOT NULL CHECK (fee_minor > 0),
  reserve_minor INTEGER NOT NULL DEFAULT 0 CHECK (reserve_minor >= 0),
  configuration_revision INTEGER NOT NULL CHECK (configuration_revision > 0),
  status TEXT NOT NULL CHECK (status IN ('active','refunded','settled')),
  prize_id TEXT,
  result_id TEXT REFERENCES activity_draw_results(id),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(chat_id, message_id)
);

CREATE TABLE activity_reward_squad_access (
  purchase_id TEXT NOT NULL REFERENCES purchases(id) ON DELETE CASCADE,
  squad_uuid TEXT NOT NULL,
  source_result_id TEXT NOT NULL REFERENCES activity_draw_results(id),
  PRIMARY KEY(purchase_id,squad_uuid,source_result_id)
);

CREATE TABLE admin_api_tokens (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 name TEXT NOT NULL CHECK(length(name) BETWEEN 1 AND 80),
 token_prefix TEXT NOT NULL,
 token_hash BLOB NOT NULL UNIQUE CHECK(length(token_hash)=32),
 scopes_json TEXT NOT NULL CHECK(json_valid(scopes_json)),
 created_at TEXT NOT NULL,
 expires_at TEXT,
 last_used_at TEXT,
 revoked_at TEXT
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
  singleton INTEGER PRIMARY KEY CHECK (singleton=1),
  config_id TEXT NOT NULL REFERENCES affiliate_config_versions(id)
);

CREATE TABLE affiliate_config_versions (
  id TEXT PRIMARY KEY,
  version INTEGER NOT NULL UNIQUE CHECK (version > 0),
  created_by TEXT REFERENCES users(id),
  created_at TEXT NOT NULL
);

CREATE TABLE affiliate_settlements (
  id TEXT PRIMARY KEY,
  invited_user_id TEXT NOT NULL UNIQUE REFERENCES users(id),
  inviter_user_id TEXT NOT NULL REFERENCES users(id),
  payment_order_id TEXT NOT NULL UNIQUE REFERENCES payment_orders(id),
  config_id TEXT NOT NULL REFERENCES affiliate_config_versions(id),
  tier_id TEXT NOT NULL REFERENCES affiliate_tiers(id),
  tier_name TEXT NOT NULL,
  commission_bps INTEGER NOT NULL CHECK (commission_bps BETWEEN 0 AND 10000),
  topup_txb_minor INTEGER NOT NULL CHECK (topup_txb_minor > 0),
  commission_txb_minor INTEGER NOT NULL CHECK (commission_txb_minor >= 0),
  settled_at TEXT NOT NULL
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
  reward_txb_minor INTEGER,
  reward_extension_days INTEGER,
  awarded_at TEXT NOT NULL,
  CHECK (
    (reward_kind='none' AND reward_coupon_id IS NULL AND reward_txb_minor IS NULL AND reward_extension_days IS NULL) OR
    (reward_kind='coupon' AND reward_coupon_id IS NOT NULL AND reward_txb_minor IS NULL AND reward_extension_days IS NULL) OR
    (reward_kind='txb' AND reward_coupon_id IS NULL AND reward_txb_minor IS NOT NULL AND reward_extension_days IS NULL) OR
    (reward_kind='subscription_extension' AND reward_coupon_id IS NULL AND reward_txb_minor IS NULL AND reward_extension_days IS NOT NULL)
  ),
  UNIQUE(inviter_user_id,tier_id)
);

CREATE TABLE affiliate_tiers (
  id TEXT PRIMARY KEY,
  config_id TEXT NOT NULL REFERENCES affiliate_config_versions(id),
  position INTEGER NOT NULL CHECK (position >= 0),
  name TEXT NOT NULL,
  success_threshold INTEGER NOT NULL CHECK (success_threshold >= 0),
  enabled INTEGER NOT NULL CHECK (enabled IN (0,1)),
  commission_enabled INTEGER NOT NULL CHECK (commission_enabled IN (0,1)),
  commission_bps INTEGER NOT NULL CHECK (commission_bps BETWEEN 0 AND 10000),
  reward_kind TEXT NOT NULL CHECK (reward_kind IN ('none','coupon','txb','subscription_extension')),
  reward_coupon_id TEXT REFERENCES coupon_definitions(id),
  reward_txb_minor INTEGER CHECK (reward_txb_minor > 0),
  reward_extension_days INTEGER CHECK (reward_extension_days BETWEEN 1 AND 3650),
  CHECK (
    (reward_kind='none' AND reward_coupon_id IS NULL AND reward_txb_minor IS NULL AND reward_extension_days IS NULL) OR
    (reward_kind='coupon' AND reward_coupon_id IS NOT NULL AND reward_txb_minor IS NULL AND reward_extension_days IS NULL) OR
    (reward_kind='txb' AND reward_coupon_id IS NULL AND reward_txb_minor IS NOT NULL AND reward_extension_days IS NULL) OR
    (reward_kind='subscription_extension' AND reward_coupon_id IS NULL AND reward_txb_minor IS NULL AND reward_extension_days IS NOT NULL)
  ),
  UNIQUE(config_id,position)
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
  size_bytes INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL CHECK (status IN ('running','complete','failed')),
  error TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  completed_at TEXT
, source TEXT NOT NULL DEFAULT 'generated', actor_user_id TEXT REFERENCES users(id) ON DELETE SET NULL, idempotency_key TEXT NOT NULL DEFAULT '', request_fingerprint TEXT NOT NULL DEFAULT '', sha256 TEXT NOT NULL DEFAULT '', original_filename TEXT NOT NULL DEFAULT '', reason TEXT NOT NULL DEFAULT '');

CREATE TABLE backup_uploads (
  id TEXT PRIMARY KEY,
  backup_run_id TEXT NOT NULL UNIQUE REFERENCES backup_runs(id) ON DELETE CASCADE,
  actor_user_id TEXT NOT NULL REFERENCES users(id),
  idempotency_key TEXT NOT NULL CHECK (length(idempotency_key) BETWEEN 1 AND 128),
  expected_sha256 TEXT NOT NULL DEFAULT '',
  actual_sha256 TEXT NOT NULL DEFAULT '',
  temporary_path TEXT NOT NULL,
  final_path TEXT NOT NULL,
  size_bytes INTEGER NOT NULL DEFAULT 0 CHECK (size_bytes >= 0),
  status TEXT NOT NULL CHECK (status IN ('receiving','validating','publishing','complete','failed')),
  last_error TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  completed_at TEXT,
  UNIQUE(actor_user_id,idempotency_key)
);

CREATE TABLE balances (
  user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  txb_minor INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL
);

CREATE TABLE browser_access_links (
  token_hash BLOB PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  return_to TEXT NOT NULL,
  expires_at TEXT NOT NULL
);

CREATE TABLE combos (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  price_txb_minor INTEGER NOT NULL CHECK (price_txb_minor >= 0),
  validity_days INTEGER NOT NULL CHECK (validity_days BETWEEN 1 AND 3650),
  traffic_limit_bytes INTEGER NOT NULL CHECK (traffic_limit_bytes > 0),
  active INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
, included_squad_uuids TEXT NOT NULL DEFAULT '[]'
  CHECK (json_valid(included_squad_uuids) AND json_type(included_squad_uuids)='array'), reset_strategy TEXT NOT NULL DEFAULT 'MONTH_ROLLING'
  CHECK (reset_strategy IN ('DAY','WEEK','MONTH_ROLLING')), ip_lookup_quota INTEGER CHECK(ip_lookup_quota IS NULL OR ip_lookup_quota BETWEEN 0 AND 1000000), whitelist_ip_quota INTEGER NOT NULL DEFAULT 5
 CHECK(whitelist_ip_quota BETWEEN 0 AND 1000));

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

CREATE TABLE "connection_scans" (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  idempotency_key TEXT NOT NULL CHECK (length(idempotency_key) BETWEEN 1 AND 128),
  request_fingerprint TEXT NOT NULL CHECK (length(request_fingerprint) BETWEEN 16 AND 128),
  provider_job_id TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'queued'
    CHECK (status IN ('queued','processing','succeeded','failed','pending_review')),
  progress_percent REAL NOT NULL DEFAULT 0
    CHECK (progress_percent >= 0 AND progress_percent <= 100),
  error_code TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  completed_at TEXT,
  expires_at TEXT NOT NULL,
  UNIQUE(user_id,idempotency_key)
);

CREATE TABLE coupon_definitions (
  id TEXT PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN ('purchase_recurring','purchase_once','balance_add','balance_multiply')),
  discount_mode TEXT NOT NULL DEFAULT '' CHECK (discount_mode IN ('','fixed','percent')),
  value_minor_or_bps INTEGER NOT NULL CHECK (value_minor_or_bps > 0),
  percent_cap_minor INTEGER CHECK (percent_cap_minor IS NULL OR percent_cap_minor > 0),
  eligible_combo_ids TEXT NOT NULL DEFAULT '[]',
  eligible_squad_ids TEXT NOT NULL DEFAULT '[]',
  expires_at TEXT,
  global_use_limit INTEGER CHECK (global_use_limit IS NULL OR global_use_limit > 0),
  per_user_use_limit INTEGER CHECK (per_user_use_limit IS NULL OR per_user_use_limit > 0),
  active INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
, admin_visible INTEGER NOT NULL DEFAULT 1 CHECK (admin_visible IN (0,1)));

CREATE TABLE coupon_grant_discards (
  grant_id TEXT PRIMARY KEY REFERENCES coupon_grants(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  discarded_at TEXT NOT NULL
);

CREATE TABLE coupon_grants (
  id TEXT PRIMARY KEY,
  coupon_id TEXT NOT NULL REFERENCES coupon_definitions(id),
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  source_type TEXT NOT NULL,
  source_id TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','consumed','expired')),
  use_count INTEGER NOT NULL DEFAULT 0 CHECK (use_count >= 0),
  created_at TEXT NOT NULL,
  consumed_at TEXT,
  UNIQUE(user_id, coupon_id, source_type, source_id)
);

CREATE TABLE coupon_redemptions (
  id TEXT PRIMARY KEY,
  coupon_id TEXT NOT NULL REFERENCES coupon_definitions(id),
  grant_id TEXT REFERENCES coupon_grants(id),
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  balance_delta_minor INTEGER NOT NULL DEFAULT 0,
  balance_after_minor INTEGER NOT NULL,
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
  gross_price_minor INTEGER NOT NULL CHECK (gross_price_minor >= 0),
  discount_minor INTEGER NOT NULL CHECK (discount_minor >= 0),
  net_price_minor INTEGER NOT NULL CHECK (net_price_minor >= 0),
  created_at TEXT NOT NULL,
  UNIQUE(purchase_id),
  UNIQUE(grant_id, purchase_id)
);

CREATE TABLE courtesy_credits (
  id TEXT PRIMARY KEY,
  payment_order_id TEXT NOT NULL UNIQUE REFERENCES payment_orders(id) ON DELETE CASCADE,
  actor_user_id TEXT NOT NULL REFERENCES users(id),
  txb_minor INTEGER NOT NULL CHECK (txb_minor > 0),
  ledger_entry_id TEXT NOT NULL UNIQUE REFERENCES ledger_entries(id),
  reason TEXT NOT NULL,
  created_at TEXT NOT NULL
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

CREATE TABLE "emby_account_disabled_folders" (
  account_id TEXT NOT NULL REFERENCES "emby_accounts"(id) ON DELETE CASCADE,
  folder_id TEXT NOT NULL,
  PRIMARY KEY (account_id,folder_id)
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
  setup_price_txb_minor INTEGER NOT NULL CHECK (setup_price_txb_minor >= 0),
  setup_attempt INTEGER NOT NULL DEFAULT 1 CHECK (setup_attempt >= 1),
  password_ciphertext TEXT NOT NULL DEFAULT '',
  password_context TEXT NOT NULL DEFAULT '',
  max_parental_rating INTEGER,
  create_attempted INTEGER NOT NULL DEFAULT 0 CHECK (create_attempted IN (0,1)),
  last_error TEXT NOT NULL DEFAULT '',
  provisioned_at TEXT,
  refunded_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  pending_preferences_json TEXT NOT NULL DEFAULT '{}'
    CHECK (json_valid(pending_preferences_json))
);

CREATE TABLE host_connectivity_attempts (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL,
    host_uuid TEXT,
    config_hash TEXT NOT NULL CHECK (length(config_hash)=64),
    proxy_user_id INTEGER NOT NULL CHECK (proxy_user_id>0),
    trigger TEXT NOT NULL CHECK (trigger IN ('manual','scheduled')),
    started_at TEXT NOT NULL CHECK (length(started_at)=30),
    finished_at TEXT CHECK (finished_at IS NULL OR (length(finished_at)=30 AND finished_at>=started_at)),
    status TEXT NOT NULL CHECK (status IN ('running','connected','failed','unsupported','error','interrupted')),
    latency_ms REAL CHECK (latency_ms IS NULL OR latency_ms>=0),
    http_status INTEGER CHECK (http_status IS NULL OR http_status BETWEEN 100 AND 599),
    error_code TEXT NOT NULL DEFAULT '',
    CHECK ((status='running' AND finished_at IS NULL AND latency_ms IS NULL AND http_status IS NULL AND error_code='')
        OR (status<>'running' AND finished_at IS NOT NULL))
);

CREATE TABLE ip_lookup_allowances (
    purchase_id TEXT PRIMARY KEY REFERENCES purchases(id) ON DELETE CASCADE,
    total INTEGER NOT NULL CHECK(total >= 0),
    used INTEGER NOT NULL DEFAULT 0 CHECK(used >= 0 AND used <= total)
);

CREATE TABLE "ip_lookup_checks" (
 id TEXT NOT NULL UNIQUE,
 operation_id TEXT PRIMARY KEY REFERENCES provider_operations(id) ON DELETE CASCADE,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 report_id TEXT REFERENCES ip_lookup_reports(id) ON DELETE SET NULL,
 allowance_id TEXT REFERENCES ip_lookup_allowances(purchase_id) ON DELETE SET NULL,
 charge_minor INTEGER NOT NULL CHECK(charge_minor >= 0),
 cached INTEGER NOT NULL CHECK(cached IN (0,1)),
 refunded INTEGER NOT NULL DEFAULT 0 CHECK(refunded IN (0,1)),
 quota_consumed INTEGER NOT NULL DEFAULT 0 CHECK(quota_consumed BETWEEN 0 AND 1000000)
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

CREATE TABLE ledger_entries (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id),
  delta_txb_minor INTEGER NOT NULL,
  balance_after INTEGER NOT NULL,
  kind TEXT NOT NULL,
  reference_id TEXT NOT NULL,
  note TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  UNIQUE(kind, reference_id)
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

CREATE TABLE member_identity_aliases (
 old_user_id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL,
 old_proxy_id INTEGER UNIQUE,
 proxy_id INTEGER UNIQUE,
 CHECK ((old_proxy_id IS NULL)=(proxy_id IS NULL))
);

CREATE TABLE node_compensation_config (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  enabled INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0,1)),
  threshold_minutes INTEGER CHECK (threshold_minutes BETWEEN 1 AND 5256000),
  multiplier_bps INTEGER CHECK (multiplier_bps BETWEEN 100 AND 1000000),
  revision INTEGER NOT NULL DEFAULT 0 CHECK (revision >= 0),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  CHECK (enabled = 0 OR (threshold_minutes IS NOT NULL AND multiplier_bps IS NOT NULL))
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

CREATE TABLE node_compensation_events (
  id TEXT PRIMARY KEY,
  node_uuid TEXT NOT NULL,
  node_name TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('observing','pending_review','queued','dismissed','ineligible')),
  offline_observed_at TEXT NOT NULL,
  recovered_observed_at TEXT,
  observed_duration_seconds INTEGER CHECK (observed_duration_seconds IS NULL OR observed_duration_seconds >= 0),
  threshold_minutes INTEGER NOT NULL CHECK (threshold_minutes BETWEEN 1 AND 5256000),
  multiplier_bps INTEGER NOT NULL CHECK (multiplier_bps BETWEEN 100 AND 1000000),
  proposed_extension_minutes INTEGER CHECK (proposed_extension_minutes IS NULL OR proposed_extension_minutes BETWEEN 0 AND 5256000),
  final_extension_minutes INTEGER CHECK (final_extension_minutes IS NULL OR final_extension_minutes BETWEEN 1 AND 5256000),
  capped INTEGER NOT NULL DEFAULT 0 CHECK (capped IN (0,1)),
  frozen_recipient_count INTEGER NOT NULL DEFAULT 0 CHECK (frozen_recipient_count >= 0),
  eligible_recipient_count INTEGER CHECK (eligible_recipient_count IS NULL OR eligible_recipient_count >= 0),
  skipped_recipient_count INTEGER CHECK (skipped_recipient_count IS NULL OR skipped_recipient_count >= 0),
  ineligible_reason TEXT CHECK (ineligible_reason IS NULL OR ineligible_reason IN ('node_disabled','below_threshold','no_recipients','computed_zero')),
  reviewed_by TEXT REFERENCES users(id),
  reviewed_at TEXT,
  review_reason TEXT,
  review_idempotency_key TEXT,
  review_fingerprint TEXT,
  provider_operation_id TEXT REFERENCES provider_operations(id),
  revision INTEGER NOT NULL DEFAULT 0 CHECK (revision >= 0),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE node_compensation_node_state (
  node_uuid TEXT PRIMARY KEY,
  node_name TEXT NOT NULL,
  is_connected INTEGER NOT NULL CHECK (is_connected IN (0,1)),
  is_disabled INTEGER NOT NULL CHECK (is_disabled IN (0,1)),
  open_event_id TEXT REFERENCES node_compensation_events(id),
  last_observed_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE onboarding_content (
  kind TEXT PRIMARY KEY CHECK (kind IN ('welcome','agreements')),
  draft_json TEXT NOT NULL CHECK (json_valid(draft_json)),
  published_json TEXT NOT NULL CHECK (json_valid(published_json)),
  draft_revision INTEGER NOT NULL DEFAULT 1 CHECK (draft_revision >= 1),
  published_revision INTEGER NOT NULL DEFAULT 1 CHECK (published_revision >= 1),
  updated_at TEXT NOT NULL,
  published_at TEXT NOT NULL,
  updated_by TEXT REFERENCES users(id)
);

CREATE TABLE "outbox_jobs" (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  payload TEXT NOT NULL CHECK (json_valid(payload)),
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','processing','done','failed')),
  attempts INTEGER NOT NULL DEFAULT 0,
  available_at TEXT NOT NULL,
  last_error TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE panel_entry_verification (
    telegram_id INTEGER PRIMARY KEY REFERENCES users(telegram_id) ON DELETE CASCADE,
    checked INTEGER NOT NULL DEFAULT 0 CHECK (checked IN (0,1))
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

CREATE TABLE "payment_orders" (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES users(id),
 provider TEXT NOT NULL CHECK(provider IN ('ezpay','bepusdt','stars','builtin_usdt')),
 status TEXT NOT NULL CHECK(status IN ('creating','pending','paid','expired','failed','refunded')),
 txb_minor INTEGER NOT NULL CHECK(txb_minor>0),
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

CREATE TABLE "payment_profiles" (
  id TEXT PRIMARY KEY,
  provider TEXT NOT NULL CHECK (provider IN ('ezpay','bepusdt')),
  provider_name TEXT NOT NULL,
  enabled_channels TEXT NOT NULL DEFAULT '',
  endpoint TEXT NOT NULL,
  merchant_id TEXT NOT NULL DEFAULT '',
  credential_ciphertext TEXT NOT NULL DEFAULT '',
  acknowledgement TEXT NOT NULL DEFAULT '',
  enabled INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0,1)),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE payment_status_rollups (
  local_date TEXT NOT NULL,
  provider TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('paid','expired','cancelled','failed','refunded')),
  order_count INTEGER NOT NULL DEFAULT 0 CHECK (order_count >= 0),
  txb_minor INTEGER NOT NULL DEFAULT 0 CHECK (txb_minor >= 0),
  updated_at TEXT NOT NULL,
  PRIMARY KEY(local_date,provider,status)
);

CREATE TABLE pricing_admin_overrides (
 id TEXT PRIMARY KEY, scope_type TEXT NOT NULL, scope_id TEXT NOT NULL,
 command TEXT NOT NULL, reason TEXT NOT NULL, created_by TEXT NOT NULL,
 starts_at TEXT NOT NULL, expires_at TEXT NOT NULL,
 expected_version INTEGER NOT NULL, previous_json TEXT NOT NULL, proposed_json TEXT NOT NULL,
 status TEXT NOT NULL CHECK(status IN ('ACTIVE','CANCELLED','EXPIRED')),
 created_at TEXT NOT NULL, CHECK(expires_at>starts_at)
);

CREATE TABLE pricing_calibration_daily (
 day TEXT NOT NULL, model_version TEXT NOT NULL, kind TEXT NOT NULL,
 bucket INTEGER NOT NULL CHECK(bucket BETWEEN 0 AND 9), samples INTEGER NOT NULL,
 probability_sum DOUBLE PRECISION NOT NULL, observed_sum INTEGER NOT NULL, brier_sum DOUBLE PRECISION NOT NULL,
 updated_at TEXT NOT NULL, PRIMARY KEY(day,model_version,kind,bucket)
);

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

CREATE TABLE pricing_cost_metadata (
 node_id TEXT PRIMARY KEY REFERENCES xnode_nodes(id), cycle_cost_minor INTEGER NOT NULL CHECK(cycle_cost_minor>=0),
 cycle_days INTEGER NOT NULL CHECK(cycle_days BETWEEN 1 AND 366),
 capacity_bytes INTEGER NOT NULL CHECK(capacity_bytes>0), effective_at TEXT NOT NULL,
 updated_by TEXT NOT NULL, updated_at TEXT NOT NULL, version INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE pricing_cost_versions (
 node_id TEXT NOT NULL REFERENCES xnode_nodes(id), version INTEGER NOT NULL,
 cycle_cost_minor INTEGER NOT NULL, cycle_days INTEGER NOT NULL, capacity_bytes INTEGER NOT NULL,
 effective_at TEXT NOT NULL, recorded_at TEXT NOT NULL, actor_id TEXT NOT NULL, reason TEXT NOT NULL,
 actual_paid_minor INTEGER CHECK(actual_paid_minor>=0), PRIMARY KEY(node_id,version)
);

CREATE TABLE pricing_decisions (
 id TEXT PRIMARY KEY, product_type TEXT NOT NULL, product_id TEXT NOT NULL,
 user_id TEXT, generated_at TEXT NOT NULL, policy_version INTEGER NOT NULL,
 model_version TEXT NOT NULL, reference_minor INTEGER NOT NULL, proposed_minor INTEGER NOT NULL,
 selected_minor INTEGER NOT NULL, state TEXT NOT NULL, trace_json TEXT NOT NULL
);

CREATE TABLE pricing_discount_spend (
 override_id TEXT NOT NULL REFERENCES pricing_admin_overrides(id), reference_id TEXT NOT NULL,
 user_id TEXT NOT NULL REFERENCES users(id), discount_minor INTEGER NOT NULL CHECK(discount_minor>=0),
 created_at TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'CONSUMED' CHECK(status IN ('RESERVED','CONSUMED','RELEASED')),
 PRIMARY KEY(override_id,reference_id)
);

CREATE TABLE pricing_features (
 feature_key INTEGER PRIMARY KEY AUTOINCREMENT, id TEXT NOT NULL UNIQUE, entity_type TEXT NOT NULL, entity_id TEXT NOT NULL,
 day TEXT NOT NULL, watermark TEXT NOT NULL, coverage_bps INTEGER NOT NULL CHECK(coverage_bps BETWEEN 0 AND 10000),
 health_state TEXT NOT NULL, features_json TEXT NOT NULL, snapshot_hash TEXT NOT NULL,
 UNIQUE(entity_type,entity_id,day)
);

CREATE TABLE pricing_forecast_actuals (
 forecast_id INTEGER PRIMARY KEY REFERENCES pricing_forecast_values(forecast_id) ON DELETE CASCADE,
 actual DOUBLE PRECISION, status_code SMALLINT NOT NULL CHECK(status_code BETWEEN 0 AND 3),
 observed_until_ns INTEGER NOT NULL, evaluated_at_ns INTEGER NOT NULL,
 CHECK((status_code=0 AND actual IS NOT NULL) OR (status_code<>0 AND actual IS NULL))
);

CREATE TABLE pricing_forecast_batches (
 batch_key INTEGER PRIMARY KEY AUTOINCREMENT,
 metric TEXT NOT NULL, generated_at TEXT NOT NULL, target_from TEXT NOT NULL,
 target_until TEXT NOT NULL, horizon_seconds INTEGER NOT NULL,
 model_version TEXT NOT NULL, health_state TEXT NOT NULL, calibration_status TEXT NOT NULL,
 CHECK(generated_at<=target_from AND target_until>target_from),
 UNIQUE(metric,generated_at,target_from,target_until,horizon_seconds,model_version,health_state,calibration_status)
);

CREATE TABLE pricing_forecast_entities (
 entity_key INTEGER PRIMARY KEY AUTOINCREMENT, entity_type TEXT NOT NULL, entity_id TEXT NOT NULL,
 UNIQUE(entity_type,entity_id)
);

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

CREATE TABLE pricing_model_validation_daily (
 day TEXT NOT NULL, model_version TEXT NOT NULL, metric TEXT NOT NULL,
 horizon_seconds INTEGER NOT NULL, samples INTEGER NOT NULL, metrics_json TEXT NOT NULL, updated_at_ns INTEGER NOT NULL,
 PRIMARY KEY(day,model_version,metric,horizon_seconds)
);

CREATE TABLE pricing_model_versions (
 id TEXT PRIMARY KEY, kind TEXT NOT NULL, created_at TEXT NOT NULL,
 calibration_status TEXT NOT NULL CHECK(calibration_status IN ('VALIDATED','UNVALIDATED','PRIOR_ONLY')),
 parameters_json TEXT NOT NULL, feature_definition_json TEXT NOT NULL,
 watermark TEXT NOT NULL, diagnostics_json TEXT NOT NULL, frozen INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE pricing_offers (
 id TEXT PRIMARY KEY, user_id TEXT NOT NULL, product_type TEXT NOT NULL, product_id TEXT NOT NULL,
 price_minor INTEGER NOT NULL, reference_minor INTEGER NOT NULL, exposed_at TEXT NOT NULL,
 expires_at TEXT NOT NULL, outcome INTEGER CHECK(outcome IN (0,1)), observed_at TEXT,
 purchase_id TEXT, model_version TEXT NOT NULL, predicted_probability DOUBLE PRECISION NOT NULL,
 features_json TEXT NOT NULL, outcome_reason TEXT NOT NULL DEFAULT '', UNIQUE(user_id,product_type,product_id,price_minor,exposed_at)
);

CREATE TABLE pricing_operational_incidents (
 id TEXT PRIMARY KEY, scope_type TEXT NOT NULL, scope_id TEXT NOT NULL,
 state TEXT NOT NULL, started_at TEXT NOT NULL, ended_at TEXT,
 reason TEXT NOT NULL, acknowledged_by TEXT, acknowledged_at TEXT
);

CREATE TABLE pricing_personal_decisions (
 version TEXT PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES users(id),
 product_type TEXT NOT NULL CHECK (product_type IN ('combo','addon','payg')),
 product_id TEXT NOT NULL,
 context_key TEXT NOT NULL,
 policy_version INTEGER NOT NULL,
 public_minor INTEGER NOT NULL CHECK(public_minor>=0),
 personal_minor INTEGER NOT NULL CHECK(personal_minor>=0),
 net_minor INTEGER NOT NULL CHECK(net_minor>=0),
 status TEXT NOT NULL CHECK(status IN ('ADJUSTED','UNCHANGED')),
 reason TEXT NOT NULL,
 model_version TEXT NOT NULL,
 probability REAL,
 calibration TEXT NOT NULL,
 generated_at TEXT NOT NULL,
 valid_until TEXT NOT NULL,
 coupon_json TEXT NOT NULL CHECK(json_valid(coupon_json))
);

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

CREATE TABLE pricing_renewal_quotes (
 id TEXT PRIMARY KEY, purchase_id TEXT NOT NULL REFERENCES purchases(id) ON DELETE CASCADE, contract_id TEXT NOT NULL REFERENCES pricing_contracts(id) ON DELETE CASCADE,
 price_minor INTEGER NOT NULL CHECK(price_minor>=0), policy_version INTEGER NOT NULL,
 contract_version INTEGER NOT NULL, state TEXT NOT NULL CHECK(state IN ('FORECASTED','PREPARED','NOTICE_PENDING','AUTHORIZED','READY_TO_SETTLE','SETTLED','DEFERRED_HEALTH','EXPIRED','CANCELLED')),
 generated_at TEXT NOT NULL, notice_at TEXT, authorized_at TEXT,
 settle_after TEXT NOT NULL, valid_until TEXT NOT NULL, settled_purchase_id TEXT,
 probability DOUBLE PRECISION NOT NULL, contribution DOUBLE PRECISION NOT NULL, health_state TEXT NOT NULL,
 model_version TEXT NOT NULL, trace_json TEXT NOT NULL, outcome_reason TEXT NOT NULL DEFAULT '',
 CHECK(valid_until>settle_after)
);

CREATE TABLE pricing_worker_watermark (singleton INTEGER PRIMARY KEY CHECK(singleton=1),observed_at TEXT NOT NULL);

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

CREATE TABLE provider_operations (
  id TEXT PRIMARY KEY,
  actor_user_id TEXT NOT NULL REFERENCES users(id),
  owner_user_id TEXT REFERENCES users(id),
  kind TEXT NOT NULL CHECK (length(kind) BETWEEN 1 AND 80),
  idempotency_key TEXT NOT NULL CHECK (length(idempotency_key) BETWEEN 1 AND 128),
  request_fingerprint TEXT NOT NULL CHECK (length(request_fingerprint) BETWEEN 16 AND 128),
  status TEXT NOT NULL DEFAULT 'queued'
    CHECK (status IN ('queued','processing','succeeded','failed','compensated','pending_review','partial')),
  attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
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

CREATE TABLE purchase_addon_adjustments (
  id TEXT PRIMARY KEY,
  purchase_id TEXT NOT NULL REFERENCES purchases(id) ON DELETE CASCADE,
  idempotency_key TEXT NOT NULL CHECK (length(idempotency_key) BETWEEN 1 AND 128),
  request_fingerprint TEXT NOT NULL CHECK (length(request_fingerprint) BETWEEN 16 AND 128),
  charged_txb_minor INTEGER NOT NULL CHECK (charged_txb_minor >= 0),
  created_at TEXT NOT NULL,
  UNIQUE(purchase_id,idempotency_key)
);

CREATE TABLE purchase_addons (
  purchase_id TEXT NOT NULL REFERENCES purchases(id) ON DELETE CASCADE,
  group_id TEXT NOT NULL,
  charged_txb_minor INTEGER NOT NULL DEFAULT 0 CHECK (charged_txb_minor >= 0),
  PRIMARY KEY (purchase_id, group_id)
);

CREATE TABLE purchase_history_tombstones (
  purchase_id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  combo_id TEXT NOT NULL,
  status TEXT NOT NULL,
  charged_txb_minor INTEGER NOT NULL CHECK (charged_txb_minor >= 0),
  gross_txb_minor INTEGER NOT NULL CHECK (gross_txb_minor >= 0),
  core_gross_txb_minor INTEGER NOT NULL CHECK (core_gross_txb_minor >= 0),
  coupon_discount_txb_minor INTEGER NOT NULL CHECK (coupon_discount_txb_minor >= 0),
  addon_count INTEGER NOT NULL DEFAULT 0 CHECK (addon_count >= 0),
  traffic_limit_bytes INTEGER NOT NULL CHECK (traffic_limit_bytes > 0),
  reset_strategy TEXT NOT NULL CHECK (reset_strategy IN ('DAY','WEEK','MONTH_ROLLING')),
  valid_from TEXT NOT NULL,
  valid_until TEXT NOT NULL,
  renewal_batch_id TEXT NOT NULL DEFAULT '',
  renewal_index INTEGER,
  auto_renew_source_purchase_id TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  removed_at TEXT NOT NULL
);

CREATE TABLE purchases (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id),
  combo_id TEXT NOT NULL REFERENCES combos(id),
  charged_txb_minor INTEGER NOT NULL CHECK (charged_txb_minor >= 0),
  valid_from TEXT NOT NULL,
  valid_until TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('activating','active','queued','expired','cancelled','failed')),
  traffic_reset_phase TEXT NOT NULL DEFAULT 'pending' CHECK (traffic_reset_phase IN ('pending','quiesced','reset')),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
, coupon_grant_id TEXT REFERENCES coupon_grants(id), gross_price_txb_minor INTEGER CHECK (gross_price_txb_minor IS NULL OR gross_price_txb_minor >= 0), coupon_discount_txb_minor INTEGER NOT NULL DEFAULT 0 CHECK (coupon_discount_txb_minor >= 0), idempotency_key TEXT, request_fingerprint TEXT NOT NULL DEFAULT '', renewal_batch_id TEXT, renewal_index INTEGER, auto_renew_enabled INTEGER NOT NULL DEFAULT 0
  CHECK (auto_renew_enabled IN (0,1)), auto_renew_failure_reason TEXT NOT NULL DEFAULT '', auto_renew_failed_at TEXT, recurring_discount_attached INTEGER NOT NULL DEFAULT 0
  CHECK (recurring_discount_attached IN (0,1)), auto_renew_source_purchase_id TEXT
  REFERENCES purchases(id), core_gross_txb_minor INTEGER NOT NULL DEFAULT 0
  CHECK (core_gross_txb_minor >= 0), entitlement_traffic_limit_bytes INTEGER
  CHECK (entitlement_traffic_limit_bytes IS NULL OR entitlement_traffic_limit_bytes > 0), entitlement_reset_strategy TEXT
  CHECK (entitlement_reset_strategy IS NULL OR entitlement_reset_strategy IN ('DAY','WEEK','MONTH_ROLLING')), entitlement_squad_uuids TEXT
  CHECK (entitlement_squad_uuids IS NULL OR (json_valid(entitlement_squad_uuids) AND json_type(entitlement_squad_uuids)='array')), entitlement_addon_squad_uuids TEXT
  CHECK (entitlement_addon_squad_uuids IS NULL OR (json_valid(entitlement_addon_squad_uuids) AND json_type(entitlement_addon_squad_uuids)='array')), reward_renewal_price_minor INTEGER CHECK (reward_renewal_price_minor IS NULL OR reward_renewal_price_minor >= 0), reward_traffic_renewal INTEGER NOT NULL DEFAULT 1 CHECK (reward_traffic_renewal IN (0,1)), reward_renewal_traffic_limit_bytes INTEGER
  CHECK (reward_renewal_traffic_limit_bytes IS NULL OR reward_renewal_traffic_limit_bytes > 0), combo_squad_uuids TEXT);

CREATE TABLE questionnaire_imports (
  id TEXT PRIMARY KEY,
  questionnaire_id TEXT NOT NULL REFERENCES questionnaires(id) ON DELETE CASCADE,
  status TEXT NOT NULL CHECK (status IN ('preview','queued','processing','settled','failed')),
  raw_csv BLOB NOT NULL,
  delimiter TEXT NOT NULL CHECK (length(delimiter) = 1),
  headers_json TEXT NOT NULL,
  sample_rows_json TEXT NOT NULL,
  data_row_count INTEGER NOT NULL CHECK (data_row_count >= 0),
  malformed_row_count INTEGER NOT NULL CHECK (malformed_row_count >= 0),
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

CREATE TABLE questionnaires (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  form_url TEXT NOT NULL,
  reward_txb_minor INTEGER NOT NULL CHECK (reward_txb_minor >= 0),
  status TEXT NOT NULL CHECK (status IN ('draft','active','closed','settling','settled')),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
, closes_at TEXT);

CREATE TABLE refunds (
  id TEXT PRIMARY KEY,
  payment_order_id TEXT NOT NULL UNIQUE REFERENCES payment_orders(id),
  actor_user_id TEXT REFERENCES users(id),
  txb_minor INTEGER NOT NULL CHECK (txb_minor > 0),
  reason TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('completed')),
  created_at TEXT NOT NULL
);

CREATE TABLE renewal_batches (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  source_purchase_id TEXT NOT NULL REFERENCES purchases(id),
  idempotency_key TEXT NOT NULL,
  request_fingerprint TEXT NOT NULL,
  term_count INTEGER NOT NULL CHECK (term_count BETWEEN 1 AND 6),
  charged_txb_minor INTEGER NOT NULL CHECK (charged_txb_minor >= 0),
  created_at TEXT NOT NULL,
  UNIQUE(user_id, idempotency_key)
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

CREATE TABLE schema_migrations (version TEXT PRIMARY KEY, applied_at TEXT NOT NULL);

CREATE TABLE sessions (
  token_hash BLOB PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at TEXT NOT NULL,
  created_at TEXT NOT NULL
, scope TEXT NOT NULL DEFAULT 'miniapp' CHECK (scope IN ('miniapp','browser')), renewed_at TEXT NOT NULL DEFAULT '');

CREATE TABLE settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  encrypted INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL,
  updated_by TEXT REFERENCES users(id)
);

CREATE TABLE squad_product_overrides (
  group_id TEXT PRIMARY KEY,
  description TEXT NOT NULL DEFAULT '',
  price_txb_minor INTEGER NOT NULL DEFAULT 0 CHECK (price_txb_minor >= 0),
  visible INTEGER NOT NULL DEFAULT 0 CHECK (visible IN (0,1)),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
, stock_limit INTEGER
  CHECK (stock_limit IS NULL OR stock_limit >= 0), profile_json TEXT
  CHECK (profile_json IS NULL OR json_valid(profile_json)), activation_required INTEGER NOT NULL DEFAULT 0
  CHECK (activation_required IN (0,1)), activation_code_hash TEXT, billing_mode TEXT NOT NULL DEFAULT 'combo' CHECK(billing_mode IN ('combo','payg')), payg_price_minor INTEGER NOT NULL DEFAULT 0 CHECK(payg_price_minor>=0 AND (billing_mode<>'payg' OR payg_price_minor>0)), payg_state TEXT NOT NULL DEFAULT 'normal'
 CHECK(payg_state IN ('normal','hidden','unavailable')));

CREATE TABLE statistics_snapshots (
  partition TEXT PRIMARY KEY CHECK (length(partition) BETWEEN 1 AND 80),
  payload_json TEXT NOT NULL
    CHECK (json_valid(payload_json) AND json_type(payload_json)='object'),
  generated_at TEXT NOT NULL
);

CREATE TABLE telegram_boost_receipts (
    chat_id INTEGER NOT NULL,
    boost_id TEXT NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (chat_id, boost_id)
);

CREATE TABLE telegram_pm_conversations (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    chat_id INTEGER NOT NULL CHECK (chat_id<0),
    topic_id INTEGER CHECK (topic_id>1),
    profile_message_id INTEGER CHECK (profile_message_id>0),
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
    expires_at INTEGER NOT NULL
);

CREATE TABLE telegram_pm_read_status (
    operation_id TEXT PRIMARY KEY REFERENCES provider_operations(id) ON DELETE CASCADE,
    read_at TEXT NOT NULL,
    source TEXT NOT NULL CHECK (source IN ('explicit','reply'))
);

CREATE TABLE telegram_pm_updates (
    update_id INTEGER PRIMARY KEY,
    created_at TEXT NOT NULL
);

CREATE TABLE txb_limits (
  singleton INTEGER PRIMARY KEY CHECK (singleton=1),
  minimum_txb_minor INTEGER NOT NULL CHECK (minimum_txb_minor > 0),
  maximum_txb_minor INTEGER NOT NULL CHECK (maximum_txb_minor >= minimum_txb_minor),
  updated_by TEXT REFERENCES users(id) ON DELETE SET NULL,
  updated_at TEXT NOT NULL
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

CREATE TABLE user_disabled_payg_squads (
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 group_id TEXT NOT NULL REFERENCES xnode_groups(id) ON DELETE CASCADE,
 PRIMARY KEY(user_id,group_id)
);

CREATE TABLE user_disabled_squads (
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 squad_uuid TEXT NOT NULL,
 PRIMARY KEY(user_id,squad_uuid)
);

CREATE TABLE user_host_names (
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 host_id TEXT NOT NULL CHECK(length(host_id) BETWEEN 1 AND 256),
 name TEXT NOT NULL CHECK(length(name) BETWEEN 1 AND 100),
 PRIMARY KEY(user_id,host_id)
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
  notify_combos INTEGER NOT NULL DEFAULT 1 CHECK (notify_combos IN (0,1)),
  notify_traffic INTEGER NOT NULL DEFAULT 1 CHECK (notify_traffic IN (0,1)),
  notify_money INTEGER NOT NULL DEFAULT 1 CHECK (notify_money IN (0,1)),
  notify_activity INTEGER NOT NULL DEFAULT 1 CHECK (notify_activity IN (0,1)),
  notify_account INTEGER NOT NULL DEFAULT 1 CHECK (notify_account IN (0,1)),
  show_referral_username INTEGER NOT NULL DEFAULT 1 CHECK (show_referral_username IN (0,1)),
  show_around_tx INTEGER NOT NULL DEFAULT 0 CHECK (show_around_tx IN (0,1)),
  show_activity INTEGER NOT NULL DEFAULT 0 CHECK (show_activity IN (0,1)),
  include_node_prices INTEGER NOT NULL DEFAULT 1 CHECK (include_node_prices IN (0,1))
, payg_enabled INTEGER NOT NULL DEFAULT 1 CHECK(payg_enabled IN (0,1)));

CREATE TABLE "users" (
  id TEXT PRIMARY KEY,
  telegram_id INTEGER NOT NULL UNIQUE,
  telegram_first_name TEXT NOT NULL DEFAULT '',
  telegram_last_name TEXT NOT NULL DEFAULT '',
  telegram_username TEXT NOT NULL DEFAULT '',
  
  role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'admin')),
  onboarding_state TEXT NOT NULL DEFAULT 'intro' CHECK (onboarding_state IN ('intro','membership','username','agreement','complete')),
  group_joined INTEGER NOT NULL DEFAULT 0,
  channel_joined INTEGER NOT NULL DEFAULT 0,
  policy_accepted_at TEXT,
  proxy_user_id TEXT UNIQUE,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
, recovery_reason TEXT NOT NULL DEFAULT '', accepted_agreement_revision INTEGER NOT NULL DEFAULT 0, new_user INTEGER NOT NULL DEFAULT 0 CHECK (new_user IN (0,1)), inviter_id INTEGER REFERENCES users(telegram_id), notification_locale TEXT NOT NULL DEFAULT 'en' CHECK (notification_locale IN ('en','zh-CN')), auto_traffic_reset_enabled INTEGER NOT NULL DEFAULT 1
  CHECK (auto_traffic_reset_enabled IN (0,1)), display_currency TEXT NOT NULL DEFAULT 'TXB'
  CHECK (display_currency IN ('TXB','CNY','USD')), pm_blocked INTEGER NOT NULL DEFAULT 0 CHECK (pm_blocked IN (0,1)), pm_muted INTEGER NOT NULL DEFAULT 0 CHECK (pm_muted IN (0,1)));

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

CREATE TABLE xnode_compile_revision (singleton INTEGER PRIMARY KEY CHECK(singleton=1), revision INTEGER NOT NULL);

CREATE TABLE xnode_config_snippets (
 config_id TEXT NOT NULL REFERENCES xnode_configs(id) ON DELETE CASCADE,
 snippet_id TEXT NOT NULL REFERENCES xnode_snippets(id), position INTEGER NOT NULL,
 PRIMARY KEY(config_id,snippet_id), UNIQUE(config_id,position)
);

CREATE TABLE xnode_configs (
 id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, document_cipher TEXT NOT NULL,
 revision INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);

CREATE TABLE "xnode_customizations" (
 proxy_user_id INTEGER NOT NULL REFERENCES xnode_identities(proxy_user_id),
 user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
 node_id TEXT NOT NULL REFERENCES xnode_nodes(id), outbounds_cipher TEXT NOT NULL, routing_cipher TEXT NOT NULL,
 revision INTEGER NOT NULL DEFAULT 1, updated_at TEXT NOT NULL, PRIMARY KEY(proxy_user_id,node_id)
);

CREATE TABLE xnode_daily_usage (
 proxy_user_id INTEGER NOT NULL REFERENCES xnode_identities(proxy_user_id),
 node_id TEXT NOT NULL REFERENCES xnode_nodes(id), period_id TEXT NOT NULL REFERENCES xnode_periods(id),
 date TEXT NOT NULL, upload_bytes INTEGER NOT NULL DEFAULT 0 CHECK(upload_bytes>=0),
 download_bytes INTEGER NOT NULL DEFAULT 0 CHECK(download_bytes>=0),
 weighted_bytes INTEGER NOT NULL DEFAULT 0 CHECK(weighted_bytes>=0),
 weight_remainder INTEGER NOT NULL DEFAULT 0 CHECK(weight_remainder BETWEEN 0 AND 999999),
 PRIMARY KEY(proxy_user_id,node_id,period_id,date)
);

CREATE TABLE xnode_ddns_changes (
 node_id TEXT NOT NULL REFERENCES xnode_nodes(id) ON DELETE CASCADE,
 job_id TEXT NOT NULL,agent_id TEXT NOT NULL,event_id TEXT NOT NULL,
 generation TEXT NOT NULL,sequence TEXT NOT NULL,
 ipv4 TEXT NOT NULL DEFAULT '',ipv6 TEXT NOT NULL DEFAULT '',observed_at TEXT NOT NULL,
 previous_ips TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(previous_ips)),
 operation_id TEXT NOT NULL REFERENCES provider_operations(id),
 phase TEXT NOT NULL CHECK(phase IN ('staging','routes','settled','obsolete')),
 PRIMARY KEY(node_id,job_id)
);

CREATE TABLE xnode_ddns_hosts (
 node_id TEXT NOT NULL,job_id TEXT NOT NULL,
 family TEXT NOT NULL CHECK(family IN ('ipv4','ipv6')),
 host_id TEXT NOT NULL UNIQUE,
 PRIMARY KEY(node_id,job_id,family,host_id),
 FOREIGN KEY(node_id,job_id) REFERENCES xnode_ddns_jobs(node_id,job_id) ON DELETE CASCADE,
 FOREIGN KEY(host_id,node_id) REFERENCES xnode_endpoints(id,node_id)
);

CREATE TABLE xnode_ddns_jobs (
 node_id TEXT NOT NULL REFERENCES xnode_nodes(id) ON DELETE CASCADE,
 job_id TEXT NOT NULL CHECK(length(job_id) BETWEEN 1 AND 128),
 configured_by TEXT NOT NULL REFERENCES users(id),
 PRIMARY KEY(node_id,job_id)
);

CREATE TABLE xnode_endpoints (
 id TEXT PRIMARY KEY, node_id TEXT NOT NULL REFERENCES xnode_nodes(id), inbound_tag TEXT NOT NULL,
 name TEXT NOT NULL, address TEXT NOT NULL, port INTEGER NOT NULL CHECK(port BETWEEN 1 AND 65535),
 enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN(0,1)), hidden INTEGER NOT NULL DEFAULT 0 CHECK(hidden IN(0,1)),
 overrides_cipher TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);

CREATE TABLE xnode_firewall_bans(
 generation TEXT PRIMARY KEY,prefix TEXT NOT NULL,ip TEXT NOT NULL,rule TEXT NOT NULL,detected_count TEXT NOT NULL,detected_at TEXT NOT NULL,
 until_at TEXT NOT NULL,refresh_seconds INTEGER NOT NULL,issued_version INTEGER NOT NULL,released_version INTEGER NOT NULL DEFAULT 0,
 released_at TEXT,participants BLOB NOT NULL,expired BLOB NOT NULL
);

CREATE TABLE xnode_firewall_clock(singleton INTEGER PRIMARY KEY CHECK(singleton=1),version INTEGER NOT NULL,source_version INTEGER NOT NULL,inventory_version INTEGER NOT NULL,active_bans INTEGER NOT NULL DEFAULT 0);

CREATE TABLE xnode_firewall_cursors(node_id TEXT NOT NULL REFERENCES xnode_nodes(id) ON DELETE CASCADE,namespace TEXT NOT NULL,sequence INTEGER NOT NULL,PRIMARY KEY(node_id,namespace));

CREATE TABLE xnode_firewall_exemptions(
 id TEXT PRIMARY KEY,user_id TEXT REFERENCES users(id),idempotency_key TEXT NOT NULL,ip TEXT NOT NULL,
 price_minor INTEGER NOT NULL,hours INTEGER NOT NULL,until_at TEXT NOT NULL,created_at TEXT NOT NULL,
 issued_version INTEGER NOT NULL,released_version INTEGER NOT NULL DEFAULT 0,ledger_id TEXT REFERENCES ledger_entries(id), grant_kind TEXT NOT NULL DEFAULT 'member'
 CHECK(grant_kind IN ('member','admin')),
 UNIQUE(user_id,idempotency_key)
);

CREATE TABLE xnode_firewall_members(
 ordinal INTEGER PRIMARY KEY AUTOINCREMENT, node_id TEXT NOT NULL UNIQUE,
 active INTEGER NOT NULL DEFAULT 1, applied INTEGER NOT NULL DEFAULT 0, source_applied INTEGER NOT NULL DEFAULT 0,
 force_sync INTEGER NOT NULL DEFAULT 1,connection_epoch INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE xnode_firewall_sources(
 id TEXT NOT NULL,generation TEXT NOT NULL,until_at TEXT NOT NULL,refresh_seconds INTEGER NOT NULL,
 issued_version INTEGER NOT NULL,released_version INTEGER NOT NULL DEFAULT 0,released_at TEXT,participants BLOB NOT NULL,expired BLOB NOT NULL,PRIMARY KEY(id,generation)
);

CREATE TABLE xnode_group_bindings (
 group_id TEXT NOT NULL REFERENCES xnode_groups(id) ON DELETE CASCADE,
 node_id TEXT NOT NULL REFERENCES xnode_nodes(id), inbound_tag TEXT NOT NULL,
 PRIMARY KEY(group_id,node_id,inbound_tag)
);

CREATE TABLE xnode_groups (
 id TEXT PRIMARY KEY, name TEXT NOT NULL, enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN(0,1)),
 created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);

CREATE TABLE xnode_identities (
 proxy_user_id INTEGER PRIMARY KEY AUTOINCREMENT,
 user_id TEXT UNIQUE REFERENCES users(id), service_name TEXT UNIQUE,
 credentials_cipher TEXT NOT NULL, subscription_hash BLOB UNIQUE,
 subscription_cipher TEXT NOT NULL DEFAULT '', current_period_id TEXT,
 legacy_user_id TEXT, lifetime_base INTEGER NOT NULL DEFAULT 0 CHECK(lifetime_base>=0),
 access_disabled INTEGER NOT NULL DEFAULT 0 CHECK(access_disabled IN(0,1)), created_at TEXT NOT NULL,
 CHECK((user_id IS NOT NULL AND service_name IS NULL) OR (user_id IS NULL AND service_name IS NOT NULL))
);

CREATE TABLE "xnode_imported_history" (
 proxy_user_id INTEGER NOT NULL REFERENCES xnode_identities(proxy_user_id),
 node_id TEXT NOT NULL, date TEXT NOT NULL,
 raw_bytes INTEGER NOT NULL CHECK(raw_bytes>=0), weighted_bytes INTEGER NOT NULL CHECK(weighted_bytes>=0),
 PRIMARY KEY(proxy_user_id,node_id,date)
);

CREATE TABLE "xnode_imported_usage" (
 proxy_user_id INTEGER PRIMARY KEY REFERENCES xnode_identities(proxy_user_id),
 period_id TEXT REFERENCES xnode_periods(id), used_bytes INTEGER NOT NULL CHECK(used_bytes>=0),
 lifetime_bytes INTEGER NOT NULL CHECK(lifetime_bytes>=0), last_reset_at TEXT, synced_at TEXT NOT NULL
);

CREATE TABLE xnode_member_diy (
 user_id TEXT PRIMARY KEY REFERENCES users(id),
 revision INTEGER NOT NULL CHECK(revision>=1 AND revision<=9007199254740991),
 cipher TEXT NOT NULL
);

CREATE TABLE xnode_metered_probes (
 proxy_user_id INTEGER PRIMARY KEY REFERENCES xnode_identities(proxy_user_id),
 period_id TEXT NOT NULL REFERENCES xnode_periods(id),
 operation_id TEXT NOT NULL REFERENCES provider_operations(id),
 pair_key TEXT NOT NULL,
 endpoint_id TEXT NOT NULL,
 node_id TEXT NOT NULL REFERENCES xnode_nodes(id),
 inbound_tag TEXT NOT NULL,
 valid_until TEXT NOT NULL,
 revoked_at TEXT,
 flushed_sequence INTEGER,
 UNIQUE(operation_id,pair_key)
);

CREATE TABLE "xnode_node_rates" (
 agent_id TEXT NOT NULL, from_sequence INTEGER NOT NULL,
 multiplier_fp INTEGER NOT NULL CHECK(multiplier_fp>=0), PRIMARY KEY(agent_id,from_sequence)
);

CREATE TABLE "xnode_nodes" (
 id TEXT PRIMARY KEY, name TEXT NOT NULL, country_code TEXT NOT NULL DEFAULT '',
 provider_name TEXT NOT NULL DEFAULT '', multiplier_fp INTEGER NOT NULL DEFAULT 1000000 CHECK(multiplier_fp>=0),
 enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0,1)),
 config_id TEXT NOT NULL REFERENCES xnode_configs(id), settings_cipher TEXT NOT NULL,
 token_hash BLOB UNIQUE CHECK(token_hash IS NULL OR length(token_hash)=32), agent_id TEXT UNIQUE,
 desired_revision INTEGER NOT NULL DEFAULT 0, accepted_revision INTEGER NOT NULL DEFAULT 0,
 applied_revision INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);

CREATE TABLE xnode_payg_accounts (
 proxy_user_id INTEGER PRIMARY KEY REFERENCES xnode_identities(proxy_user_id),
 user_id TEXT NOT NULL REFERENCES users(id), group_id TEXT NOT NULL,
 node_id TEXT NOT NULL REFERENCES xnode_nodes(id),
 period_id TEXT NOT NULL UNIQUE REFERENCES xnode_periods(id),
 unit_price_minor INTEGER NOT NULL CHECK(unit_price_minor>0),
 purchased_bytes INTEGER NOT NULL DEFAULT 0 CHECK(purchased_bytes>=0), credit_bytes INTEGER NOT NULL DEFAULT 0 CHECK(credit_bytes>=0), credit_started_at TEXT,
 UNIQUE(user_id,group_id,node_id)
);

CREATE TABLE xnode_payg_charges (
 ledger_id TEXT PRIMARY KEY REFERENCES ledger_entries(id),
 proxy_user_id INTEGER NOT NULL REFERENCES xnode_payg_accounts(proxy_user_id),
 first_block INTEGER NOT NULL CHECK(first_block>=0),
 blocks INTEGER NOT NULL CHECK(blocks>0),
 unit_price_minor INTEGER NOT NULL CHECK(unit_price_minor>0),
 UNIQUE(proxy_user_id,first_block)
);

CREATE TABLE xnode_payg_minutes (
 user_id TEXT NOT NULL REFERENCES users(id), group_id TEXT NOT NULL,
 minute TEXT NOT NULL, traffic_bytes INTEGER NOT NULL DEFAULT 0 CHECK(traffic_bytes>=0),
 spent_minor INTEGER NOT NULL DEFAULT 0 CHECK(spent_minor>=0),
 PRIMARY KEY(user_id,group_id,minute)
);

CREATE TABLE xnode_payg_windows (
 proxy_user_id INTEGER NOT NULL REFERENCES xnode_payg_accounts(proxy_user_id),
 through TEXT NOT NULL,
 traffic_bytes INTEGER NOT NULL DEFAULT 0 CHECK(traffic_bytes>=0),
 unbilled_bytes INTEGER NOT NULL DEFAULT 0 CHECK(unbilled_bytes>=0 AND unbilled_bytes<=traffic_bytes),
 PRIMARY KEY(proxy_user_id,through)
);

CREATE TABLE xnode_period_aliases (
 node_id TEXT NOT NULL REFERENCES xnode_nodes(id),
 historical_period_id TEXT NOT NULL,
 canonical_period_id TEXT NOT NULL REFERENCES xnode_periods(id),
 PRIMARY KEY(node_id,historical_period_id),
 CHECK(historical_period_id <> canonical_period_id)
);

CREATE TABLE xnode_period_nodes (
 period_id TEXT NOT NULL REFERENCES xnode_periods(id), node_id TEXT NOT NULL REFERENCES xnode_nodes(id),
 flushed_sequence INTEGER, PRIMARY KEY(period_id,node_id)
);

CREATE TABLE xnode_periods (
 id TEXT PRIMARY KEY, proxy_user_id INTEGER NOT NULL REFERENCES xnode_identities(proxy_user_id),
 purchase_id TEXT REFERENCES purchases(id), started_at TEXT NOT NULL, ended_at TEXT,
 limit_bytes INTEGER NOT NULL CHECK(limit_bytes>=0), strategy TEXT NOT NULL,
 last_reset_at TEXT, used_bytes INTEGER NOT NULL DEFAULT 0 CHECK(used_bytes>=0),
 status TEXT NOT NULL DEFAULT 'open' CHECK(status IN('open','closing','closed','review'))
);

CREATE TABLE xnode_report_cursors (
 agent_id TEXT PRIMARY KEY, node_id TEXT NOT NULL REFERENCES xnode_nodes(id),
 last_sequence INTEGER NOT NULL DEFAULT 0, last_digest TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL
);

CREATE TABLE xnode_routing_plans(
 proxy_user_id INTEGER PRIMARY KEY REFERENCES xnode_identities(proxy_user_id) ON DELETE CASCADE,
 cipher TEXT NOT NULL, previous_cipher TEXT NOT NULL DEFAULT '', legacy_cipher TEXT NOT NULL DEFAULT '',
 revision INTEGER NOT NULL CHECK(revision>0), phase TEXT NOT NULL CHECK(phase IN('revoking','applying','enabling','active','disabled')),
 operation_id TEXT NOT NULL REFERENCES provider_operations(id), reason TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL
);

CREATE TABLE xnode_service_bindings (
 proxy_user_id INTEGER NOT NULL REFERENCES xnode_identities(proxy_user_id),
 node_id TEXT NOT NULL REFERENCES xnode_nodes(id), inbound_tag TEXT NOT NULL,
 valid_until TEXT NOT NULL, PRIMARY KEY(proxy_user_id,node_id,inbound_tag)
);

CREATE TABLE xnode_snippets (
 id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, kind TEXT NOT NULL CHECK(kind IN ('outbound','routing')),
 document_cipher TEXT NOT NULL, revision INTEGER NOT NULL DEFAULT 1,
 created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);

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

CREATE INDEX pricing_active_commands ON pricing_admin_overrides(command,status,expires_at);

CREATE INDEX pricing_contract_product ON pricing_contracts(product_id,recorded_at);

CREATE INDEX pricing_contracts_user ON pricing_contracts(user_id,purchase_id);

CREATE INDEX pricing_decisions_product ON pricing_decisions(product_type,product_id,generated_at);

CREATE INDEX pricing_features_entity_day ON pricing_features(entity_type,entity_id,day DESC);

CREATE INDEX pricing_features_hash ON pricing_features(snapshot_hash,watermark);

CREATE INDEX pricing_forecasts_target ON pricing_forecast_batches(target_until,batch_key);

CREATE INDEX pricing_incidents_scope ON pricing_operational_incidents(scope_type,scope_id,started_at);

CREATE INDEX pricing_offers_expiry ON pricing_offers(expires_at,id);

CREATE INDEX pricing_offers_observed ON pricing_offers(exposed_at,model_version);

CREATE INDEX pricing_offers_product_window ON pricing_offers(product_type,product_id,exposed_at DESC,id DESC);

CREATE INDEX pricing_overrides_scope ON pricing_admin_overrides(scope_type,scope_id,status,expires_at);

CREATE INDEX pricing_personal_owner_idx ON pricing_personal_decisions(user_id,product_type,product_id,generated_at);

CREATE INDEX pricing_prices_product ON pricing_published_prices(product_type,product_id,created_at);

CREATE INDEX pricing_renewal_purchase ON pricing_renewal_quotes(purchase_id,generated_at);

CREATE INDEX pricing_values_feature ON pricing_forecast_values(feature_key);

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

CREATE INDEX xnode_ddns_changes_operation_idx ON xnode_ddns_changes(operation_id);

CREATE UNIQUE INDEX xnode_endpoint_ownership_idx ON xnode_endpoints(id,node_id);

CREATE INDEX xnode_endpoints_node_idx ON xnode_endpoints(node_id,inbound_tag);

CREATE UNIQUE INDEX xnode_firewall_active_prefix ON xnode_firewall_bans(prefix) WHERE released_version=0;

CREATE UNIQUE INDEX xnode_firewall_active_source ON xnode_firewall_sources(id) WHERE released_version=0;

CREATE INDEX xnode_firewall_ban_versions ON xnode_firewall_bans(issued_version,released_version);

CREATE INDEX xnode_firewall_exemption_versions ON xnode_firewall_exemptions(issued_version,released_version);

CREATE INDEX xnode_legacy_history_date_node_idx ON "xnode_imported_history"(date,node_id);

CREATE INDEX xnode_metered_probes_period ON xnode_metered_probes(period_id,node_id);

CREATE INDEX xnode_payg_minutes_expiry ON xnode_payg_minutes(minute);

CREATE INDEX xnode_payg_node ON xnode_payg_accounts(node_id,user_id);

CREATE INDEX xnode_payg_windows_expiry ON xnode_payg_windows(through);

CREATE INDEX xnode_periods_purchase_idx ON xnode_periods(purchase_id);

CREATE INDEX xnode_periods_user_idx ON xnode_periods(proxy_user_id,started_at);

CREATE INDEX xnode_service_bindings_expiry_idx ON xnode_service_bindings(valid_until);

CREATE TRIGGER affiliate_config_versions_no_delete BEFORE DELETE ON affiliate_config_versions BEGIN SELECT RAISE(ABORT,'affiliate config history is immutable'); END;

CREATE TRIGGER affiliate_config_versions_no_update
BEFORE UPDATE ON affiliate_config_versions
WHEN NOT (
  OLD.id IS NEW.id
  AND OLD.version IS NEW.version
  AND OLD.created_at IS NEW.created_at
  AND EXISTS (
    SELECT 1 FROM member_identity_aliases a
    WHERE (OLD.created_by=a.old_user_id
           AND NEW.created_by IN (a.user_id,'~tg:'||a.user_id))
       OR (OLD.created_by='~tg:'||a.user_id AND NEW.created_by=a.user_id)
  )
)
BEGIN
  SELECT RAISE(ABORT,'affiliate config history is immutable');
END;

CREATE TRIGGER affiliate_tiers_no_delete BEFORE DELETE ON affiliate_tiers BEGIN SELECT RAISE(ABORT,'affiliate tier history is immutable'); END;

CREATE TRIGGER affiliate_tiers_no_update BEFORE UPDATE ON affiliate_tiers BEGIN SELECT RAISE(ABORT,'affiliate tier history is immutable'); END;

CREATE TRIGGER compile_admin_temporary_bans_delete AFTER DELETE ON admin_temporary_bans BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_admin_temporary_bans_insert AFTER INSERT ON admin_temporary_bans BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_admin_temporary_bans_update AFTER UPDATE ON admin_temporary_bans BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_combo_whitelist_update AFTER UPDATE OF whitelist_ip_quota ON combos
 BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_ddns_hosts_delete AFTER DELETE ON xnode_ddns_hosts BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_ddns_hosts_insert AFTER INSERT ON xnode_ddns_hosts BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_ddns_hosts_update AFTER UPDATE ON xnode_ddns_hosts BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_ddns_jobs_delete AFTER DELETE ON xnode_ddns_jobs BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_ddns_jobs_insert AFTER INSERT ON xnode_ddns_jobs BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_ddns_jobs_update AFTER UPDATE ON xnode_ddns_jobs BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_disabled_payg_delete AFTER DELETE ON user_disabled_payg_squads BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_disabled_payg_insert AFTER INSERT ON user_disabled_payg_squads BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_payg_account_insert AFTER INSERT ON xnode_payg_accounts BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_payg_preference_delete AFTER DELETE ON user_preferences WHEN OLD.payg_enabled=0 BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_payg_preference_insert AFTER INSERT ON user_preferences WHEN NEW.payg_enabled=0 BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_payg_preference_update AFTER UPDATE OF payg_enabled ON user_preferences WHEN OLD.payg_enabled<>NEW.payg_enabled BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_payg_product_delete AFTER DELETE ON squad_product_overrides BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_payg_product_insert AFTER INSERT ON squad_product_overrides BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_payg_product_update AFTER UPDATE ON squad_product_overrides BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_purchases_delete AFTER DELETE ON purchases BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_purchases_insert AFTER INSERT ON purchases BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_purchases_update AFTER UPDATE ON purchases BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_routing_templates_insert AFTER INSERT ON settings WHEN NEW.key='xnode.routing_templates' BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_routing_templates_update AFTER UPDATE ON settings WHEN NEW.key='xnode.routing_templates' BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_settings_delete AFTER DELETE ON settings WHEN OLD.key='xnode.global' BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_settings_insert AFTER INSERT ON settings WHEN NEW.key='xnode.global' BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_settings_update AFTER UPDATE ON settings WHEN NEW.key='xnode.global' BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_user_disabled_squads_delete AFTER DELETE ON user_disabled_squads BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_user_disabled_squads_insert AFTER INSERT ON user_disabled_squads BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_user_disabled_squads_update AFTER UPDATE ON user_disabled_squads BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_users_delete AFTER DELETE ON users BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_users_insert AFTER INSERT ON users BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_users_update AFTER UPDATE OF onboarding_state ON users WHEN OLD.onboarding_state IS NOT NEW.onboarding_state BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_config_snippets_delete AFTER DELETE ON xnode_config_snippets BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_config_snippets_insert AFTER INSERT ON xnode_config_snippets BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_config_snippets_update AFTER UPDATE ON xnode_config_snippets BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_configs_delete AFTER DELETE ON xnode_configs BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_configs_insert AFTER INSERT ON xnode_configs BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_configs_update AFTER UPDATE ON xnode_configs BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_customizations_delete AFTER DELETE ON xnode_customizations BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_customizations_insert AFTER INSERT ON xnode_customizations BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_customizations_update AFTER UPDATE ON xnode_customizations BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_endpoints_delete AFTER DELETE ON xnode_endpoints BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_endpoints_insert AFTER INSERT ON xnode_endpoints BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_endpoints_update AFTER UPDATE OF node_id,inbound_tag,name,address,port,enabled,hidden,overrides_cipher ON xnode_endpoints
WHEN OLD.node_id IS NOT NEW.node_id OR OLD.inbound_tag IS NOT NEW.inbound_tag OR OLD.name IS NOT NEW.name OR OLD.address IS NOT NEW.address OR OLD.port IS NOT NEW.port OR OLD.enabled IS NOT NEW.enabled OR OLD.hidden IS NOT NEW.hidden OR OLD.overrides_cipher IS NOT NEW.overrides_cipher
BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_group_bindings_delete AFTER DELETE ON xnode_group_bindings BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_group_bindings_insert AFTER INSERT ON xnode_group_bindings BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_group_bindings_update AFTER UPDATE ON xnode_group_bindings BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_groups_delete AFTER DELETE ON xnode_groups BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_groups_insert AFTER INSERT ON xnode_groups BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_groups_update AFTER UPDATE ON xnode_groups BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_identities_delete AFTER DELETE ON xnode_identities BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_identities_insert AFTER INSERT ON xnode_identities BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_identities_update AFTER UPDATE OF credentials_cipher,current_period_id,access_disabled ON xnode_identities WHEN OLD.credentials_cipher IS NOT NEW.credentials_cipher OR OLD.current_period_id IS NOT NEW.current_period_id OR OLD.access_disabled IS NOT NEW.access_disabled BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_node_catalog_metadata AFTER UPDATE OF name,country_code,provider_name,multiplier_fp ON xnode_nodes
WHEN OLD.name IS NOT NEW.name OR OLD.country_code IS NOT NEW.country_code OR OLD.provider_name IS NOT NEW.provider_name OR OLD.multiplier_fp IS NOT NEW.multiplier_fp
BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_nodes_delete AFTER DELETE ON xnode_nodes BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_nodes_insert AFTER INSERT ON xnode_nodes BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_nodes_update AFTER UPDATE OF config_id,settings_cipher,enabled ON xnode_nodes WHEN OLD.config_id IS NOT NEW.config_id OR OLD.settings_cipher IS NOT NEW.settings_cipher OR OLD.enabled IS NOT NEW.enabled BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_periods_delete AFTER DELETE ON xnode_periods BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_periods_insert AFTER INSERT ON xnode_periods BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_periods_update AFTER UPDATE OF status,limit_bytes,strategy,used_bytes ON xnode_periods WHEN OLD.status IS NOT NEW.status OR OLD.limit_bytes IS NOT NEW.limit_bytes OR OLD.strategy IS NOT NEW.strategy OR (OLD.limit_bytes>0 AND OLD.used_bytes>=OLD.limit_bytes) IS NOT (NEW.limit_bytes>0 AND NEW.used_bytes>=NEW.limit_bytes) BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_probe_insert AFTER INSERT ON xnode_metered_probes BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_probe_revoke AFTER UPDATE OF revoked_at ON xnode_metered_probes BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_routing_insert AFTER INSERT ON xnode_routing_plans BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_routing_update AFTER UPDATE ON xnode_routing_plans BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_service_bindings_delete AFTER DELETE ON xnode_service_bindings BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_service_bindings_insert AFTER INSERT ON xnode_service_bindings BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_service_bindings_update AFTER UPDATE ON xnode_service_bindings BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_snippets_delete AFTER DELETE ON xnode_snippets BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_snippets_insert AFTER INSERT ON xnode_snippets BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER compile_xnode_snippets_update AFTER UPDATE ON xnode_snippets BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;

CREATE TRIGGER firewall_exemption_inventory_insert AFTER INSERT ON xnode_firewall_exemptions
BEGIN UPDATE xnode_firewall_clock SET inventory_version=version WHERE singleton=1; END;

CREATE TRIGGER firewall_exemption_inventory_release AFTER UPDATE OF released_version ON xnode_firewall_exemptions
WHEN OLD.released_version=0 AND NEW.released_version>0
BEGIN UPDATE xnode_firewall_clock SET inventory_version=version WHERE singleton=1; END;

CREATE TRIGGER firewall_inventory_insert AFTER INSERT ON xnode_firewall_bans WHEN NEW.released_version=0 BEGIN UPDATE xnode_firewall_clock SET inventory_version=version,active_bans=active_bans+1 WHERE singleton=1; END;

CREATE TRIGGER firewall_inventory_release AFTER UPDATE OF released_version ON xnode_firewall_bans WHEN OLD.released_version=0 AND NEW.released_version>0 BEGIN UPDATE xnode_firewall_clock SET inventory_version=version,active_bans=active_bans-1 WHERE singleton=1; END;

CREATE TRIGGER preference_visibility_after_transition
AFTER UPDATE OF status,valid_from,valid_until ON purchases
BEGIN
  UPDATE user_preferences SET show_activity=0
  WHERE user_id=NEW.user_id
    AND NOT EXISTS (SELECT 1 FROM xnode_payg_accounts WHERE user_id=NEW.user_id AND purchased_bytes>0)
    AND NOT EXISTS (
    SELECT 1 FROM purchases WHERE user_id=NEW.user_id AND status='active'
      AND julianday(valid_from)<=julianday(NEW.updated_at)
      AND julianday(valid_until)>julianday(NEW.updated_at)
  );
END;

CREATE TRIGGER preference_visibility_before_purchase
BEFORE INSERT ON purchases
BEGIN
  UPDATE user_preferences SET show_activity=0
  WHERE user_id=NEW.user_id
    AND NOT EXISTS (SELECT 1 FROM xnode_payg_accounts WHERE user_id=NEW.user_id AND purchased_bytes>0)
    AND NOT EXISTS (
    SELECT 1 FROM purchases WHERE user_id=NEW.user_id AND status='active'
      AND julianday(valid_from)<=julianday(NEW.created_at)
      AND julianday(valid_until)>julianday(NEW.created_at)
  );
END;

CREATE TRIGGER preference_visibility_before_transition
BEFORE UPDATE OF status,valid_from,valid_until ON purchases
BEGIN
  UPDATE user_preferences SET show_activity=0
  WHERE user_id=NEW.user_id
    AND NOT EXISTS (SELECT 1 FROM xnode_payg_accounts WHERE user_id=NEW.user_id AND purchased_bytes>0)
    AND NOT EXISTS (
    SELECT 1 FROM purchases WHERE user_id=NEW.user_id AND status='active'
      AND julianday(valid_from)<=julianday(NEW.updated_at)
      AND julianday(valid_until)>julianday(NEW.updated_at)
  );
END;

CREATE TRIGGER pricing_contracts_immutable BEFORE UPDATE ON pricing_contracts BEGIN SELECT RAISE(ABORT,'pricing basis snapshots are immutable'); END;

CREATE TRIGGER pricing_cost_versions_immutable BEFORE UPDATE ON pricing_cost_versions BEGIN SELECT RAISE(ABORT,'original provider cost facts are immutable'); END;

CREATE TRIGGER pricing_feature_inputs_immutable BEFORE UPDATE ON pricing_features BEGIN SELECT RAISE(ABORT,'original pricing forecast inputs are immutable'); END;

CREATE TRIGGER pricing_forecast_batches_immutable BEFORE UPDATE ON pricing_forecast_batches BEGIN SELECT RAISE(ABORT,'original pricing forecast batch metadata is immutable'); END;

CREATE TRIGGER pricing_forecast_entities_immutable BEFORE UPDATE ON pricing_forecast_entities BEGIN SELECT RAISE(ABORT,'original forecast entity is immutable'); END;

CREATE VIEW pricing_forecast_evaluations AS SELECT forecast_id,actual,CASE status_code WHEN 0 THEN 'COMPLETE' WHEN 1 THEN 'MISSING' WHEN 2 THEN 'CENSORED' ELSE 'INCOMPLETE' END AS status,observed_until_ns,evaluated_at_ns FROM pricing_forecast_actuals;

CREATE VIEW pricing_forecast_vintages AS
 SELECT v.forecast_id,e.entity_type,e.entity_id,b.metric,b.generated_at,b.target_from,b.target_until,b.horizon_seconds,
 v.predicted_p10,v.predicted_p50,v.predicted_p90,v.predicted_mean,b.model_version,
 f.watermark AS feature_watermark,f.snapshot_hash AS feature_snapshot_hash,v.data_coverage_bps,b.health_state,b.calibration_status
 FROM pricing_forecast_values v JOIN pricing_forecast_entities e ON e.entity_key=v.entity_key
 JOIN pricing_forecast_batches b ON b.batch_key=v.batch_key JOIN pricing_features f ON f.feature_key=v.feature_key;

CREATE TRIGGER pricing_forecasts_immutable BEFORE UPDATE ON pricing_forecast_values BEGIN SELECT RAISE(ABORT,'original pricing forecasts are immutable'); END;

CREATE TRIGGER pricing_model_parameters_immutable BEFORE UPDATE OF kind,created_at,parameters_json,feature_definition_json,watermark,diagnostics_json,calibration_status ON pricing_model_versions BEGIN SELECT RAISE(ABORT,'original model parameters are immutable'); END;

CREATE TRIGGER pricing_offer_inputs_immutable BEFORE UPDATE OF user_id,product_type,product_id,price_minor,reference_minor,exposed_at,expires_at,model_version,predicted_probability,features_json ON pricing_offers BEGIN SELECT RAISE(ABORT,'original price exposures are immutable'); END;

CREATE TRIGGER pricing_prices_immutable BEFORE UPDATE ON pricing_published_prices BEGIN SELECT RAISE(ABORT,'published pricing facts are immutable'); END;

CREATE TRIGGER pricing_quote_terms_immutable BEFORE UPDATE OF price_minor,contract_id,policy_version,contract_version,generated_at,settle_after,valid_until,model_version,trace_json ON pricing_renewal_quotes BEGIN SELECT RAISE(ABORT,'renewal quote terms are immutable'); END;

CREATE TRIGGER provider_operation_items_open_host_guard
BEFORE INSERT ON provider_operation_items
WHEN NEW.target_type='remnawave_host'
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
  )
BEGIN
  SELECT RAISE(ABORT,'open host remark operation exists');
END;

CREATE TRIGGER retire_xnode_firewall_member AFTER DELETE ON xnode_nodes BEGIN UPDATE xnode_firewall_members SET active=0 WHERE node_id=OLD.id; END;

CREATE TRIGGER usdt_terminal_evidence_immutable BEFORE UPDATE ON usdt_transfers
WHEN OLD.status IN ('credited','dismissed') BEGIN
 SELECT RAISE(ABORT,'terminal transfer evidence is immutable');
END;

CREATE TRIGGER user_notification_events_immutable
BEFORE UPDATE OF user_id,source_kind,source_id,kind,payload_json,gate_key,created_at
ON user_notification_events
WHEN NOT (
  OLD.source_kind IS NEW.source_kind
  AND OLD.source_id IS NEW.source_id
  AND OLD.kind IS NEW.kind
  AND OLD.payload_json IS NEW.payload_json
  AND OLD.gate_key IS NEW.gate_key
  AND OLD.created_at IS NEW.created_at
  AND EXISTS (
    SELECT 1 FROM member_identity_aliases a
    WHERE (OLD.user_id=a.old_user_id
           AND NEW.user_id IN (a.user_id,'~tg:'||a.user_id))
       OR (OLD.user_id='~tg:'||a.user_id AND NEW.user_id=a.user_id)
  )
)
BEGIN
  SELECT RAISE(ABORT,'user notification payloads are immutable');
END;

