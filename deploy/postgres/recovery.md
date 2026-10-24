# PostgreSQL recovery operations

`Dockerfile` adds pgBackRest to the official PostgreSQL 18 image.
`compose.recovery.yml` overlays `compose.yml` with continuous WAL archiving and
a separate repository volume. `pgbackrest.conf.example` configures AES-256-CBC
repository encryption; copy it to the protected `secrets/pgbackrest_config`,
replace the passphrase and grant the container PostgreSQL user read access.
Use the same private-parent/readable-file policy as the database secrets.
Never commit this configuration or print the passphrase in command output.

Start with both Compose files and the `postgres` profile. Run these commands as
the PostgreSQL OS user inside the database container:

```sh
pgbackrest --config=/run/secrets/pgbackrest_config --stanza=vansail stanza-create
pgbackrest --config=/run/secrets/pgbackrest_config --stanza=vansail check
pgbackrest --config=/run/secrets/pgbackrest_config --stanza=vansail --type=full backup
pgbackrest --config=/run/secrets/pgbackrest_config --stanza=vansail info
```

Schedule one full backup weekly and one differential backup daily using the host
scheduler, with an exclusive host lock around backup/check/expire commands.
Retain at least two full backup chains; configure longer retention from the
actual recovery requirement. pgBackRest retains the required WAL with each
chain. Monitor the repository filesystem, PostgreSQL volume and archive errors;
alert below 20% free space, on any archive failure, and if the newest successful
backup exceeds 26 hours. Test restoration after configuration/version changes.
Archive timeout is 60 seconds; an RPO claim also requires measured successful
archive latency and working storage. A local Docker volume alone is not off-site
protection. `pgbackrest-s3.conf.example` adds an independently encrypted S3
repository; validate credentials, retention and restore against the selected
provider before production use. Cloudflare R2 has not been selected or validated.

Keep a separately secured recovery bundle containing the exact application
image digest, schema version, bootstrap configuration, CONFIG_MASTER_KEY and
repository credentials. Access to a database backup without its original
CONFIG_MASTER_KEY does not recover encrypted application secrets. Preserve source
SQLite snapshots and hashed reports independently until the cutover is accepted.

## Offline restore

1. Block authoritative ingress using the cutover procedure. Stop the application,
   scheduler, outbox and X-Node master; verify their processes are stopped. Keep
   agent journals durable and preserve provider callbacks using the reviewed
   ingress mechanism. Never restore into a database used by active workers.
2. Preserve the failed database and its WAL. Select a backup with `info`, secure
   the verification evidence, and create a **new** PostgreSQL volume/deployment.
   Do not use `--delta` against the only surviving production volume.
3. With the new PostgreSQL server stopped and its fresh PGDATA owned by postgres,
   run pgBackRest restore as the PostgreSQL OS user. Use `--type=immediate` for
   the base backup's earliest consistent state, or `--type=time --target=...`
   for a reviewed timestamp. A tested named restore point can use
   `--type=name --target=...`. Set `--target-action=promote` only for the chosen
   recovery target. Mount the encrypted repository and its configuration.
4. Start the isolated database. Check recovery has completed and inspect server
   errors. Run `db-schema verify`, then `db-migrate recovery-check
   --expected-hash <secured-evidence>` with the original CONFIG_MASTER_KEY. For
   PITR, compare evidence captured at the selected point, then reconcile effects
   after it. Verify balances/ledger, sessions, encrypted settings, cursors,
   periods, usage and outbox state before starting application workers.
5. Reconcile uncertain provider effects with their supported APIs and reconcile
   unacknowledged agent journals. Re-establish backup archiving on the promoted
   timeline and take a fresh full backup before normal traffic resumes.

Logical `.dump` exports are administrative snapshots produced by `pg_dump` and
listed through the existing backup API. Restore with PostgreSQL 18 `pg_restore
--exit-on-error` into a separate empty database, using a recovery administrator
for extensions/ownership and then restoring application grants. The live API
refuses PostgreSQL archive upload and file-swap restore. A completed archive
record proves creation/structural inspection only; successful restore drills are
separate operational evidence.

Hosted CI runs `scripts/ci/postgresql_recovery.sh` with synthetic financial,
session, encrypted configuration and X-Node journal fixtures. It restores an
encrypted physical backup into two fresh isolated volumes and verifies both base
recovery and a named PITR target, comparing every original table hash. The same
pipeline restores an administrative logical export. Live recovery readiness
still requires the selected production storage and recovery bundle.

Upstream: [pgBackRest guide](https://pgbackrest.org/user-guide.html),
[PostgreSQL PITR](https://www.postgresql.org/docs/18/continuous-archiving.html).
