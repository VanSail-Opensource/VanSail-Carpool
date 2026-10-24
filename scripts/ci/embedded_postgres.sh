#!/usr/bin/env bash
# Hosted-only production-image rehearsal. Synthetic configuration; no network.
set -euo pipefail
trap 'echo "Embedded image check failed at line $LINENO" >&2' ERR
[[ ${GITHUB_ACTIONS:-} == true ]] || { echo 'Hosted CI only' >&2; exit 1; }
image=${1:?image required}
container="vansail-embedded-${GITHUB_RUN_ID}-${GITHUB_RUN_ATTEMPT}"
data="${container}-data"
pgdata="${container}-postgres"
repository="${container}-backup"
guard="${container}-guard"
guarddata="${container}-guard-data"
cleanup() {
  docker logs --tail 100 "$container" || true
  docker logs --tail 30 "$guard" 2>/dev/null || true
  docker rm -f "$container" "$guard" || true
  docker volume rm "$data" "$pgdata" "$repository" "$guarddata" || true
}
trap cleanup EXIT
docker volume create "$data"
docker volume create "$pgdata"
docker volume create "$repository"
secret="$RUNNER_TEMP/embedded-pgbackrest.conf"
cp deploy/embedded/pgbackrest.conf.example "$secret"
sed -i "s/REPLACE_WITH_INDEPENDENT_RECOVERY_SECRET/$(openssl rand -hex 32)/" "$secret"
chmod 0444 "$secret"
docker run -d --name "$container" --network none --memory 1g --memory-swap 1g \
  --read-only --cap-drop ALL --security-opt no-new-privileges:true \
  -v "$data:/data" -v "$pgdata:/var/lib/postgresql" \
  -v "$repository:/var/lib/pgbackrest" -v "$secret:/run/secrets/pgbackrest_config:ro" \
  -e PG_EMBEDDED=true -e DB_DRIVER=postgres \
  -e PGBACKREST_CONFIG=/run/secrets/pgbackrest_config \
  -e CONFIG_MASTER_KEY=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA= \
  -e ADMIN_TELEGRAM_ID=912001 -e TELEGRAM_BOT_TOKEN=123456:hosted-rehearsal-only \
  -e PUBLIC_BASE_URL=https://rehearsal.invalid "$image"
healthy() {
  for attempt in $(seq 1 90); do
    if docker exec "$container" vansail-carpool healthcheck --url http://127.0.0.1:8080/healthz; then return; fi
    [[ $(docker inspect -f '{{.State.Running}}' "$container") == true ]] || return 1
    sleep 2
  done
  return 1
}
sql() {
  docker exec -e PGHOST=/var/lib/postgresql/socket "$container" \
    psql -U postgres -d vansail -v ON_ERROR_STOP=1 -Atc "$1"
}
healthy
[[ $(docker inspect -f '{{.Config.User}}' "$container") == 10001:10001 ]]
[[ $(docker inspect -f '{{.HostConfig.Memory}}' "$container") == 1073741824 ]]
[[ $(sql "SELECT current_setting('shared_buffers')='128MB'
  AND current_setting('work_mem')='2MB'
  AND current_setting('max_connections')='32'
  AND current_setting('fsync')='on'
  AND current_setting('synchronous_commit')='on'
  AND current_setting('full_page_writes')='on'
  AND current_setting('archive_mode')='on'
  AND current_setting('listen_addresses')='127.0.0.1'") == t ]]
[[ $(sql "SELECT phase FROM vansail_migration_state") == active ]]
[[ $(sql "SELECT NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole FROM pg_roles WHERE rolname='vansail_app'") == t ]]
[[ $(sql "SELECT pg_get_userbyid(relowner) FROM pg_class WHERE oid='balances'::regclass") == vansail_migration ]]
docker logs "$container" > "$RUNNER_TEMP/embedded-startup.log" 2>&1
grep -Eq 'go_limit_bytes["=: ]+402653184' "$RUNNER_TEMP/embedded-startup.log"
sql "INSERT INTO settings(key,value,updated_at) VALUES ('ci.embedded_canary','preserved',vansail_now())"
docker stop -t 75 "$container"
[[ $(docker inspect -f '{{.State.ExitCode}}' "$container") != 137 ]]
# Physical recovery contains role hashes, not the separate plaintext URL files.
# Rehearse lost credentials in this stopped, disposable CI volume only.
docker run --rm --network none --read-only --user 10001:10001 --entrypoint sh \
  -v "$pgdata:/var/lib/postgresql" "$image" -ec '
    rm /var/lib/postgresql/credentials/application_password \
      /var/lib/postgresql/credentials/migration_password \
      /var/lib/postgresql/credentials/postgres_password
  '
docker start "$container"
healthy
[[ $(sql "SELECT value FROM settings WHERE key='ci.embedded_canary'") == preserved ]]
docker exec "$container" pgbackrest --stanza=vansail --type=full backup
docker exec "$container" pgbackrest --stanza=vansail check
# Restore into a different path, never over the active database, then query it.
docker exec "$container" pgbackrest --stanza=vansail \
  --pg1-path=/var/lib/pgbackrest/restore --type=immediate restore
docker exec "$container" /usr/libexec/postgresql18/pg_ctl \
  -D /var/lib/pgbackrest/restore -l /dev/stdout -w \
  -o '-c archive_mode=off -c port=55432' start
restored=$(docker exec -e PGHOST=/var/lib/postgresql/socket "$container" \
  psql -p 55432 -U postgres -d vansail -Atc "SELECT value FROM settings WHERE key='ci.embedded_canary'")
[[ $restored == preserved ]]
docker exec "$container" /usr/libexec/postgresql18/pg_ctl \
  -D /var/lib/pgbackrest/restore -m fast -w stop
# PostgreSQL failure must also stop the Go process/container, without a restart loop.
docker exec "$container" /usr/libexec/postgresql18/pg_ctl \
  -D /var/lib/postgresql/18/docker -m immediate -w stop
for attempt in $(seq 1 15); do
  [[ $(docker inspect -f '{{.State.Running}}' "$container") == false ]] && break
  sleep 1
done
[[ $(docker inspect -f '{{.State.Running}}' "$container") == false ]]
[[ $(docker inspect -f '{{.State.ExitCode}}' "$container") != 0 ]]
# An offline-initialized empty database must not bypass the SQLite source guard.
docker volume create "$guarddata"
start_guard() {
  docker run -d --name "$guard" --network none --memory 1g --memory-swap 1g \
    --read-only --cap-drop ALL --security-opt no-new-privileges:true \
    -v "$data:/data" -v "$guarddata:/var/lib/postgresql" \
    -e PG_EMBEDDED=true -e DB_DRIVER=postgres "$image" "$@"
}
start_guard postgres-only
for attempt in $(seq 1 30); do
  if docker exec "$guard" test -s /var/lib/postgresql/credentials/migration_url; then break; fi
  sleep 1
done
docker exec -e DATABASE_URL_FILE=/var/lib/postgresql/credentials/migration_url "$guard" db-schema migrate
docker exec "$guard" sh -ec 'set -C; printf synthetic-legacy-source > /data/tx-carpool.db'
docker stop -t 75 "$guard"
docker rm "$guard"
start_guard serve
for attempt in $(seq 1 30); do
  [[ $(docker inspect -f '{{.State.Running}}' "$guard") == false ]] && break
  sleep 1
done
[[ $(docker inspect -f '{{.State.Running}}' "$guard") == false ]]
[[ $(docker inspect -f '{{.State.ExitCode}}' "$guard") != 0 ]]
docker logs "$guard" > "$RUNNER_TEMP/embedded-guard.log" 2>&1
grep -q 'Existing SQLite requires verified import' "$RUNNER_TEMP/embedded-guard.log"
echo 'Verified 1 GiB embedded PostgreSQL startup, tuning, persistence, encrypted recovery, signals and failure supervision.'
