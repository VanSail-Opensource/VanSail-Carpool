#!/bin/sh
set -eu
PGPASSWORD=$(cat /run/secrets/application_password)
export PGPASSWORD
psql -h 127.0.0.1 -U vansail_app -d vansail --set ON_ERROR_STOP=1 <<'SQL'
SELECT 1 / ((current_user = 'vansail_app')::int);
SELECT 1 / ((NOT has_schema_privilege(current_user, 'public', 'CREATE'))::int);
SELECT 1 / ((current_setting('password_encryption') = 'scram-sha-256')::int);
SQL
