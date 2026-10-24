#!/bin/sh
set -eu
MONITOR_PASSWORD=$(cat /run/secrets/monitoring_password)
export MONITOR_PASSWORD
psql --username postgres --dbname vansail --set ON_ERROR_STOP=1 <<'SQL'
\getenv monitor_password MONITOR_PASSWORD
ALTER ROLE vansail_monitor LOGIN PASSWORD :'monitor_password';
GRANT SELECT ON vansail_outbox_observability,vansail_financial_observability,vansail_accounting_observability,vansail_backup_observability TO vansail_monitor;
SQL
unset MONITOR_PASSWORD
