# Process configuration

`config.go` validates the public origin, identities, encrypted-settings master
key and runtime directories. `database.go` accepts PostgreSQL only, defaulting
DB_DRIVER to postgres. Provide DATABASE_URL or a protected DATABASE_URL_FILE;
embedded deployments supply this internally. Invalid/missing connections fail
clearly without logging URL credentials. Pools and all SQL/connect/lock deadlines
are bounded; DB_MIGRATE_ON_START defaults to false for the restricted app role.

`memory.go` derives GOMEMLIMIT from the container and resolves protected process
scratch under DATA_DIR/tmp. Explicit runtime limits remain authoritative.
`memory_cgroup.go` reads container/ancestor limits. `config_test.go`,
`database_test.go` and `memory_test.go` cover these bootstrap contracts in hosted
CI. Scratch remains available for assets/decoders; transient inventories live in
session-scoped PostgreSQL tables. No database file path or upload cap is configured.

`memory_observe.go` publishes bounded runtime/metrics summaries without profiles or secrets.
