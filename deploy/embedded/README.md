# Embedded PostgreSQL deployment

The production image includes PostgreSQL 18 server, client/contrib tools and
pgBackRest. `PG_EMBEDDED=true DB_DRIVER=postgres` opts into running the server
inside the application container. Legacy SQLite and explicitly configured
external PostgreSQL remain supported. Never automatically convert a live source.

`compose.yml` is the selected deployment: one unprivileged UID 10001 container,
1 GiB memory/swap ceiling, read-only root, dropped capabilities, `/data` preserved,
and a separate durable volume at `/var/lib/postgresql`. Only HTTP is published;
PostgreSQL listens on loopback and a private Unix socket. Use a reviewed image
digest and the protected `deploy/postgres/secrets/application.env` containing
the existing bot, origin, administrator and master-key configuration. Do not
put database URLs in this file for embedded mode.

`entrypoint.sh` runs under Tini, starts PostgreSQL before schema work, runs
`db-schema migrate` with the migration role, then launches the restricted
application role. Signals stop the application first, then perform a fast,
durable PostgreSQL shutdown. Either process failing stops the whole container.
`postgres.sh` creates a UTF-8/C cluster, SCRAM roles and random passwords only
when required. Secrets/URLs persist with mode 0600 below the database volume;
the shared role initializer is restart-safe. A populated SQLite source prevents
an accidental fresh PostgreSQL application startup.
If recovery lacks the separate password files, peer-authenticated bootstrap
recreates and synchronizes SQL login passwords before starting the application;
the application master key must still come from independent protected recovery.

For an existing installation, follow the [cutover gates](../postgres/cutover.md).
Start this container with command `postgres-only` while the application is
stopped, then run offline `db-migrate` commands using
`DATABASE_URL_FILE=/var/lib/postgresql/credentials/migration_url`. After import,
verification and cutover-check, recreate with the normal `serve` command. The
application's activation gate rechecks imported hashes before workers can run.
Do not run the external PostgreSQL Compose service against this same volume.

`tuning.sh` budgets for the combined container rather than a dedicated database:
at 1 GiB, 128 MiB shared buffers, 256 MiB effective cache estimate, 2 MiB per sort,
32 MiB maintenance, two 16 MiB autovacuum workers and 32 connections. Go derives
a 384 MiB soft heap limit unless GOMEMLIMIT is explicitly supplied. The 8/4 app
pool leaves room for ownership/listener, migrations, backups and diagnostics.
WAL compression, 10-minute spread checkpoints, 512 MiB WAL target, lock logging
and pg_stat_statements are enabled. fsync, synchronous_commit and full_page_writes
stay enabled. WAL size is a target, not a hard disk limit; alert on disk space
and archiver failures. These are bounded starting settings; production latency,
pool waits and query plans still require measurement before workload tuning.

PostgreSQL logs use container stdout and the bounded Docker log rotation. For
physical backup/WAL archiving, copy `pgbackrest.conf.example` into protected
`deploy/postgres/secrets/embedded_pgbackrest_config`, replace the cipher secret,
and merge `compose.recovery.yml`. It mounts the independently protected config
and repository volume, sets `PGBACKREST_CONFIG`, and uses the
[recovery procedure](../postgres/recovery.md), adjusting the container command
and keeping the same PGDATA. Archive configuration is generated at startup.
Keep the repository cipher key and CONFIG_MASTER_KEY independently backed up.
Logical exports use the same installed client tools. SQL roles separate DDL
from application access; processes share one OS user and filesystem, so this is
not an OS-level security boundary between the application and database.

Hosted image checks exercise fresh startup, tuning/durability, restricted roles,
data persistence on restart, signals, encrypted physical backup and isolated
restoration, and database-process failure. They do not
replace the protected production-data rehearsal.

References: [PostgreSQL memory](https://www.postgresql.org/docs/18/runtime-config-resource.html),
[durability/checkpoints](https://www.postgresql.org/docs/18/runtime-config-wal.html),
[Alpine packaging](https://github.com/alpinelinux/aports/blob/master/main/postgresql18/APKBUILD).
