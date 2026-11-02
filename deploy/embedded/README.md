# Embedded PostgreSQL deployment

The production image includes PostgreSQL 18 server, client/contrib tools and
pgBackRest. `PG_EMBEDDED=true DB_DRIVER=postgres` opts into running the server
inside the application container. Explicitly configured external PostgreSQL
is also supported; disable PG_EMBEDDED and provide its protected connection URL.

`compose.yml` is the selected deployment: one unprivileged UID 10001 container,
1 GiB memory/swap ceiling, read-only root, dropped capabilities, `/data` preserved,
and a separate durable volume at `/var/lib/postgresql`. Only HTTP is published;
PostgreSQL listens on loopback and a private Unix socket. Use a reviewed image
digest and the protected `deploy/postgres/secrets/application.env` containing
the existing bot, origin, administrator and master-key configuration. Do not
put database URLs in this file for embedded mode.

Always bind both `/data` and `/var/lib/postgresql` to explicit persistent volumes,
as this Compose file does. `/data` holds logical backups and temporary application assets. Disposable proxy
inventories use session-scoped PostgreSQL temporary tables, without durable rows.
An omitted mount creates an anonymous volume that can be abandoned on recreation;
repeated deployments can leave several copies of the cache and backup history.
To repair an existing deployment, inspect the current container mounts and declare
the existing `/data` volume as an external Compose volume before recreating it.
Do not replace it with an empty volume or prune volumes containing backups.

Database size excludes WAL, proxy inventories, exports, old Docker volumes and
images. Diagnose these separately with `docker system df`, container mount
inspection and filesystem sizes. Remove only reviewed, unused disposable caches;
preserve database volumes, migration evidence and logical exports. Never unlink PostgreSQL files or run a blanket volume prune as maintenance.

`entrypoint.sh` runs under Tini, starts PostgreSQL before schema work, runs
`db-schema migrate` with the migration role, then launches the restricted
application role. Signals stop the application first, then perform a fast,
durable PostgreSQL shutdown. Either process failing stops the whole container.
`postgres.sh` creates a UTF-8/C cluster, SCRAM roles and random passwords only
when required. Secrets/URLs persist with mode 0600 below the database volume;
the shared role initializer is restart-safe. Existing clusters are reused and
major-version mismatches fail rather than reinitializing the database.
If recovery lacks the separate password files, peer-authenticated bootstrap
recreates and synchronizes SQL login passwords before starting the application;
the application master key must still come from independent protected recovery.

For upgrades, retain the current PostgreSQL volume and application master key.
`db-schema migrate` applies only new forward migrations; existing SQL scripts and
historical records remain unchanged. `compose.offline.yml` starts only PostgreSQL
for operator maintenance/recovery with workers stopped. It is not an import path.
Never run the external server against the same data volume or use `down -v`.

`tuning.sh` budgets for the combined container rather than a dedicated database:
at 1 GiB, 128 MiB shared buffers, 256 MiB effective cache estimate, 2 MiB per sort,
32 MiB maintenance, two 16 MiB autovacuum workers and 32 connections. Go derives
a 384 MiB soft heap limit unless GOMEMLIMIT is explicitly supplied. The 8/8 app
pool leaves room for ownership/listener, migrations, backups and diagnostics.
Keeping up to all eight application connections idle avoids repeated SCRAM
authentication and backend-process creation during short reconciliation bursts.
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
