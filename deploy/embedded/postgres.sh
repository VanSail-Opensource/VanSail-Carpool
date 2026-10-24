#!/bin/sh
# Sourced only by entrypoint.sh, as the unprivileged application OS user.
embedded_start() {
  credentials=/var/lib/postgresql/credentials
  credentials_created=false
  for role in postgres application migration; do
    if [ ! -s "$credentials/${role}_password" ]; then
      # Hex passwords need no URL escaping. Never replace an existing secret.
      (set -C; od -An -N32 -tx1 /dev/urandom | tr -d ' \n' > "$credentials/${role}_password")
      credentials_created=true
    fi
  done
  if [ ! -f "$PGDATA/PG_VERSION" ]; then
    if [ -s "${DATA_DIR:-/data}/tx-carpool.db" ] && [ "$1" != postgres-only ]; then
      echo 'Existing SQLite detected: use postgres-only and the verified offline migration procedure first' >&2
      exit 1
    fi
    initdb -D "$PGDATA" --username=postgres --pwfile="$credentials/postgres_password" \
      --encoding=UTF8 --locale=C --auth-host=scram-sha-256 --auth-local=peer
  fi
  [ "$(cat "$PGDATA/PG_VERSION")" = 18 ] || { echo 'PostgreSQL major version mismatch' >&2; exit 1; }
  cat > "$PGDATA/pg_ident.conf" <<'IDENT'
vansail_owner txcarpool postgres
IDENT
  cat > "$PGDATA/pg_hba.conf" <<'HBA'
local all postgres peer map=vansail_owner
host all all 127.0.0.1/32 scram-sha-256
HBA
  . /usr/local/lib/vansail/tuning.sh
  embedded_tuning > "$PGDATA/vansail.conf"
  # Include once; operator overrides remain in postgresql.auto.conf.
  grep -q "^include = 'vansail.conf'" "$PGDATA/postgresql.conf" || \
    echo "include = 'vansail.conf'" >> "$PGDATA/postgresql.conf"
  pg_ctl -D "$PGDATA" -w -t 60 -l /dev/stdout start
  if [ ! -f "$PGDATA/vansail-initialized" ] || [ "$credentials_created" = true ]; then
    if ! psql -d postgres -Atc "SELECT 1 FROM pg_database WHERE datname='vansail'" | grep -qx 1; then
      createdb --owner=postgres vansail
    fi
    POSTGRES_USER=postgres POSTGRES_DB=vansail \
      APP_PASSWORD_FILE="$credentials/application_password" \
      MIGRATION_PASSWORD_FILE="$credentials/migration_password" \
      sh /usr/local/lib/vansail/init-roles.sh
    # A restored cluster may lack the separate plaintext secrets. Peer-authenticated
    # bootstrap repairs role passwords before the app starts; data stays intact.
    PG_OWNER_PASSWORD=$(cat "$credentials/postgres_password")
    export PG_OWNER_PASSWORD
    psql -v ON_ERROR_STOP=1 <<'SQL'
\getenv owner_password PG_OWNER_PASSWORD
ALTER ROLE postgres PASSWORD :'owner_password';
SQL
    unset PG_OWNER_PASSWORD
    touch "$PGDATA/vansail-initialized"
  fi
  for role in application migration; do
    username=vansail_app
    [ "$role" != migration ] || username=vansail_migration
    printf 'postgres://%s:%s@127.0.0.1:5432/vansail?sslmode=disable\n' \
      "$username" "$(cat "$credentials/${role}_password")" > "$credentials/${role}_url"
  done
  # Physical/WAL recovery is opt-in with independently protected repository keys.
  if [ -n "${PGBACKREST_CONFIG:-}" ]; then
    pgbackrest --stanza=vansail stanza-create
  fi
}
