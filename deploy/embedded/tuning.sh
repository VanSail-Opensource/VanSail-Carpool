#!/bin/sh
embedded_tuning() {
  # Bound shared memory for a combined Go/DB container; never use dedicated-DB ratios.
  memory_mb=1024
  for limit_file in /sys/fs/cgroup/memory.max /sys/fs/cgroup/memory/memory.limit_in_bytes; do
    if [ -r "$limit_file" ]; then
      candidate=$(awk '$1 ~ /^[0-9]+$/ && $1 < 1152921504606846976 {printf "%.0f", $1/1048576}' "$limit_file")
      if [ -n "$candidate" ] && [ "$candidate" -lt "$memory_mb" ]; then memory_mb=$candidate; fi
    fi
  done
  shared_mb=$((memory_mb / 8))
  [ "$shared_mb" -ge 32 ] || { echo 'Embedded PostgreSQL requires at least 256 MiB' >&2; return 1; }
  cat <<SETTINGS
listen_addresses = '127.0.0.1'
unix_socket_directories = '$PGHOST'
unix_socket_permissions = 0700
password_encryption = 'scram-sha-256'
shared_preload_libraries = 'pg_stat_statements'
max_connections = 32
shared_buffers = '${shared_mb}MB'
effective_cache_size = '$((memory_mb / 4))MB'
work_mem = '2MB'
hash_mem_multiplier = 1
maintenance_work_mem = '32MB'
autovacuum_work_mem = '16MB'
autovacuum_max_workers = 2
max_parallel_workers_per_gather = 0
max_wal_size = '512MB'
min_wal_size = '80MB'
checkpoint_timeout = '10min'
checkpoint_completion_target = 0.9
wal_compression = on
fsync = on
synchronous_commit = on
full_page_writes = on
idle_in_transaction_session_timeout = '30s'
log_checkpoints = on
log_lock_waits = on
log_min_duration_statement = -1
logging_collector = off
SETTINGS
  if [ -n "${PGBACKREST_CONFIG:-}" ]; then
    cat <<'ARCHIVE'
archive_mode = on
archive_command = 'pgbackrest --stanza=vansail archive-push %p'
archive_timeout = '60s'
ARCHIVE
  fi
}
