# PostgreSQL infrastructure

See [recovery operations](recovery.md) for the pgBackRest image, encrypted local
and S3 repository examples, offline restore, retention and hosted recovery drills.

This opt-in Compose project starts PostgreSQL independently. It does not switch
the application database. PostgreSQL 18 stores its versioned PGDATA below the
volume mounted at `/var/lib/postgresql`, per the official image contract.

1. Create `secrets/` with mode 0700. Generate three different passwords into
   `postgres_password`, `application_password`, and `migration_password`, mode
   0444 inside that private directory. Compose file secrets are bind mounts;
   PostgreSQL's container OS user must be able to read them. The 0700 parent
   protects them on the host. These files are ignored by Git. Use managed Docker
   secrets or a secret manager in production.
2. Run `docker compose -f deploy/postgres/compose.yml --profile postgres up -d`.
3. Check health and authenticate using the application role from a container
   joined to `vansail-postgres_database`. No database port is published.

`init.sh` runs only for an empty volume. The application role has DML rights and
sequence use; the migration role owns schema changes. Neither is a superuser.
Physical backup/WAL operations run as the database OS owner in the recovery
integration rather than handing superuser credentials to the application.

Use a protected `DATABASE_URL_FILE` for application credentials. Explicitly set
`DB_DRIVER=postgres`; keep startup migration disabled for the restricted app role.
Run migrations separately with the migration role. Keep `/data` and the app
container's existing read-only root, capability drop and resource restrictions.
Attach the app to the private database network with a healthy-service dependency
only after database compatibility and migration verification gates pass.

Configuration validates pool bounds and positive duration deadlines. Defaults:
8 open / 4 idle connections, 30m lifetime, 10s connect, 30s statement, 5s lock.
`DB_MIGRATE_ON_START` defaults to true for legacy SQLite and false for PostgreSQL.
Database failures must stop startup; they must never select the other backend.

References: [official image](https://hub.docker.com/_/postgres),
[PostgreSQL authentication](https://www.postgresql.org/docs/18/auth-password.html).
