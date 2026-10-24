#!/bin/sh
set -eu
if [ "${PG_EMBEDDED:-false}" != true ]; then
  exec /usr/local/bin/vansail-carpool "$@"
fi
case "${1:-}" in
  serve|postgres-only) ;;
  *) exec /usr/local/bin/vansail-carpool "$@" ;;
esac
if [ "${DB_DRIVER:-}" != postgres ]; then
  echo 'PG_EMBEDDED=true requires DB_DRIVER=postgres' >&2
  exit 1
fi
if [ -n "${DATABASE_URL:-}${DATABASE_URL_FILE:-}" ]; then
  echo 'Embedded PostgreSQL manages its own URL; remove DATABASE_URL and DATABASE_URL_FILE' >&2
  exit 1
fi
export PATH="/usr/libexec/postgresql18:$PATH"
export PGDATA=/var/lib/postgresql/18/docker
export PGHOST=/var/lib/postgresql/socket PGUSER=postgres PGDATABASE=vansail
umask 077
mkdir -p "$PGHOST" /var/lib/postgresql/credentials
chmod 700 "$PGHOST" /var/lib/postgresql/credentials
app_pid=
cleanup() {
  trap '' TERM INT
  if [ -n "$app_pid" ]; then
    kill -TERM "$app_pid" 2>/dev/null || true
    wait "$app_pid" || true
  fi
  pg_ctl -D "$PGDATA" -m fast -w -t 30 stop || true
}
trap cleanup EXIT
trap 'exit 143' TERM
trap 'exit 130' INT
. /usr/local/lib/vansail/postgres.sh
embedded_start "$1"
if [ "$1" = postgres-only ]; then
  while pg_ctl -D "$PGDATA" status >/dev/null 2>&1; do sleep 1 & wait $!; done
  exit 1
fi
# Schema work uses migration credentials before the restricted app is started.
DATABASE_URL_FILE=/var/lib/postgresql/credentials/migration_url db-schema migrate
if [ -s "${DATA_DIR:-/data}/tx-carpool.db" ]; then
  phase=$(psql -Atc 'SELECT phase FROM vansail_migration_state WHERE singleton=1')
  if [ "$phase" = fresh ]; then
    echo 'Existing SQLite requires verified import before starting the PostgreSQL application' >&2
    exit 1
  fi
fi
export DATABASE_URL_FILE=/var/lib/postgresql/credentials/application_url
export DB_MIGRATE_ON_START=false
/usr/local/bin/vansail-carpool "$@" &
app_pid=$!
# Exit the whole container if either process dies. Tini reaps PostgreSQL children.
while kill -0 "$app_pid" 2>/dev/null; do
  if ! pg_ctl -D "$PGDATA" status >/dev/null 2>&1; then exit 1; fi
  sleep 1 & wait $!
done
result=0
wait "$app_pid" || result=$?
app_pid=
exit "$result"
