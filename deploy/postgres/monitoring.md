# PostgreSQL monitoring

Merge `compose.monitoring.yml` into the database stack. Create a separate
protected `secrets/monitoring_password`, apply database migrations, then run
`sh /opt/vansail/monitoring-init.sh` as the PostgreSQL OS user in the database
container. Start only `postgres-exporter` with the `monitoring` profile after
schema/import/recovery work. It has no published port and uses the private
database network, a read-only filesystem and no Linux capabilities.

`init.sh` creates a NOLOGIN monitoring role with `pg_monitor`. The explicit
initialization enables password login and grants SELECT on aggregate views only;
it does not grant SELECT on financial records, settings, sessions or credentials.
For an existing cluster, provision this role with those same privileges before
running the script. Use migration-role ownership for views and restrict access to
the monitoring network. The application/migration credentials are separate.

`exporter-queries.yml` adds derived queue state/age, retained attempts, exact
financial discrepancies, known accounting barriers and logical export age.
`prometheus.example.yml` and `alerts.yml` can be loaded into an existing
Prometheus; Grafana can query that source without another application database.
The pinned exporter also exposes locks, deadlocks, long transactions, WAL
archiver failures and normalized pg_stat_statements timings. Query text labels
are disabled. Financial reconciliation scans ledger totals: measure scrape cost
on the actual installation and adjust cadence/deadlines rather than persisting a
second financial source of truth.

The application logs pool utilization/waits, transaction time, retry counts and
commit-to-ACK timing every 30 seconds. Derive rates from these monotonic process
counters in the existing log pipeline. Counts reset on process restart. No SQL,
URL, payload, nonce, source IP or user labels are emitted. Transaction timing
covers BeginTx through Commit/Rollback; statement timing comes from PostgreSQL,
and upstream queue timing remains owned by the existing request queues.

The current X-Node public protocol does not expose total agent journal depth.
Accounting metrics cover durable known flush barriers and unfinished periods;
combine them with existing agent journal telemetry when assessing unseen replay
backlog. A zero known backlog does not prove an agent's offline journal is empty.

Physical backup status comes from pgBackRest, independently of logical exports.
Pipe `pgbackrest --config=/run/secrets/pgbackrest_config --stanza=vansail
--output=json info` into `backup-metrics.py` and atomically publish its output in
the existing node_exporter textfile directory. Do this after every backup/check
and periodically from the single host scheduler. `alerts.yml` includes backup
age; alert additionally when `vansail_pgbackrest_status` is nonzero or expected
metrics disappear. Monitor repository/database disk capacity separately.

Redis/Valkey, PgBouncer, sqlc and extra database extensions remain deferred. This
initial rollout has one application process and retains the existing in-memory
nonce/rate-limit contracts. Redis failure therefore has no effect on durable work.

Upstream: [exporter v0.20.1 configuration](https://github.com/prometheus-community/postgres_exporter/blob/v0.20.1/README.md),
[PostgreSQL statistics](https://www.postgresql.org/docs/18/monitoring-stats.html).
