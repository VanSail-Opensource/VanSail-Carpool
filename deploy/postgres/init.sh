#!/bin/sh
set -eu
# psql variable interpolation quotes secrets as SQL literals; never enable xtrace.
APP_PASSWORD=$(cat /run/secrets/application_password)
MIGRATION_PASSWORD=$(cat /run/secrets/migration_password)
export APP_PASSWORD MIGRATION_PASSWORD
psql --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" --set ON_ERROR_STOP=1 <<'SQL'
\getenv app_password APP_PASSWORD
\getenv migration_password MIGRATION_PASSWORD
CREATE ROLE vansail_app LOGIN PASSWORD :'app_password';
CREATE ROLE vansail_migration LOGIN PASSWORD :'migration_password';
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT CONNECT ON DATABASE vansail TO vansail_app, vansail_migration;
GRANT USAGE ON SCHEMA public TO vansail_app;
GRANT USAGE, CREATE ON SCHEMA public TO vansail_migration;
ALTER DEFAULT PRIVILEGES FOR ROLE vansail_migration IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO vansail_app;
ALTER DEFAULT PRIVILEGES FOR ROLE vansail_migration IN SCHEMA public
  GRANT USAGE, SELECT ON SEQUENCES TO vansail_app;
CREATE EXTENSION pg_stat_statements;
SQL
unset APP_PASSWORD MIGRATION_PASSWORD
