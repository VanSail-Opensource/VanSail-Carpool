#!/usr/bin/env bash
# Hosted synthetic fixture only. Restores use fresh, isolated Docker volumes.
set -euo pipefail
if [[ ${GITHUB_ACTIONS:-} != true ]]; then
  echo 'This drill is restricted to hosted CI.' >&2
  exit 1
fi
container=$(docker compose ps -q postgres)
image=$(docker inspect --format '{{.Config.Image}}' "$container")
repo=$(docker inspect --format '{{range .Mounts}}{{if eq .Destination "/var/lib/pgbackrest"}}{{.Name}}{{end}}{{end}}' "$container")
config=$(realpath deploy/postgres/secrets/pgbackrest_config)
evidence=$(docker compose exec -T postgres sh -c '
  password=$(cat /run/secrets/migration_password)
  export DB_DRIVER=postgres DATABASE_URL="postgres://vansail_migration:$password@127.0.0.1/vansail?sslmode=disable"
  /tmp/db-migrate cutover-check
' | python3 -c 'import json,sys; print(json.load(sys.stdin)["sha256"])')

docker compose exec -T --user postgres postgres psql -v ON_ERROR_STOP=1 -d vansail <<'SQL'
CREATE TABLE recovery_canary(id bigint PRIMARY KEY);
INSERT INTO recovery_canary VALUES(1);
SQL
docker compose exec -T --user postgres postgres pgbackrest --config=/run/secrets/pgbackrest_config --stanza=vansail stanza-create
docker compose exec -T --user postgres postgres pgbackrest --config=/run/secrets/pgbackrest_config --stanza=vansail check
docker compose exec -T --user postgres postgres pgbackrest --config=/run/secrets/pgbackrest_config --stanza=vansail --type=full backup
docker compose exec -T --user postgres postgres psql -v ON_ERROR_STOP=1 -d vansail <<'SQL'
INSERT INTO recovery_canary VALUES(2);
SELECT pg_create_restore_point('vansail_verified_recovery');
INSERT INTO recovery_canary VALUES(3);
SELECT pg_switch_wal();
SQL
docker compose exec -T --user postgres postgres pgbackrest --config=/run/secrets/pgbackrest_config --stanza=vansail check

restore_one() {
  local kind=$1 expected=$2
  local volume="vansail_ci_${GITHUB_RUN_ID}_${kind}" name="vansail-ci-${GITHUB_RUN_ID}-${kind}"
  docker volume create "$volume" >/dev/null
  docker run --rm --network none --user root --entrypoint sh \
    --mount "type=volume,src=$volume,dst=/var/lib/postgresql" \
    --mount "type=volume,src=$repo,dst=/var/lib/pgbackrest,readonly" \
    --mount "type=bind,src=$config,dst=/run/secrets/pgbackrest_config,readonly" \
    "$image" -ec '
      install -d -m 0700 -o postgres -g postgres /var/lib/postgresql/18/docker
      if [ "$1" = full ]; then
        exec gosu postgres pgbackrest --config=/run/secrets/pgbackrest_config --stanza=vansail --type=immediate --target-action=promote restore
      fi
      exec gosu postgres pgbackrest --config=/run/secrets/pgbackrest_config --stanza=vansail --type=name --target=vansail_verified_recovery --target-action=promote restore
    ' sh "$kind"
  docker run -d --name "$name" --network none \
    --mount "type=volume,src=$volume,dst=/var/lib/postgresql" \
    --mount "type=volume,src=$repo,dst=/var/lib/pgbackrest,readonly" \
    --mount "type=bind,src=$config,dst=/run/secrets/pgbackrest_config,readonly" \
    "$image" postgres -c archive_mode=off >/dev/null
  local ready=false
  for attempt in {1..60}; do
    if docker exec --user postgres "$name" psql -At -d vansail -c 'SELECT NOT pg_is_in_recovery()' 2>/dev/null | grep -qx t; then ready=true; break; fi
    sleep 2
  done
  if [[ $ready != true ]]; then docker logs "$name"; return 1; fi
  actual=$(docker exec --user postgres "$name" psql -At -d vansail -c "SELECT string_agg(id::text,',' ORDER BY id) FROM recovery_canary")
  if [[ $actual != "$expected" ]]; then echo "Recovery target differs: $kind $actual" >&2; return 1; fi
  docker cp "$RUNNER_TEMP/db-migrate" "$name:/tmp/db-migrate"
  docker exec -e DB_DRIVER=postgres \
    -e "DATABASE_URL=postgres://vansail_migration:$(cat deploy/postgres/secrets/migration_password)@127.0.0.1/vansail?sslmode=disable" \
    -e CONFIG_MASTER_KEY=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA= \
    "$name" /tmp/db-migrate recovery-check --expected-hash "$evidence"
  docker stop "$name" >/dev/null
  docker rm "$name" >/dev/null
  docker volume rm "$volume" >/dev/null
}
restore_one full 1
restore_one pitr 1,2
echo 'Verified encrypted full backup and named-target PITR against all 127 exact table hashes.'
