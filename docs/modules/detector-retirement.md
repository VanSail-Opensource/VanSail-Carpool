# Detector retirement

The X-Node control plane replaces the legacy QPS/abuse detector. Detector ingestion,
rules, punishments, node credentials, APIs, navigation, translations, notification
jobs and installer are removed. IP Lookup risk providers and member/admin IP-block
controls are separate features and remain available.

Migration `078_retire_abuse_detector.sql` cancels pending/processing detector jobs
and deletes operational detector tables, encrypted node credentials and raw samples.
Historical incident, delivery, financial and audit rows remain available through the
existing database/audit facilities. Old migrations stay immutable for upgrades.

For acknowledged historical temporary bans, one durable compatibility job per user
releases the retired restriction. It preserves manual restrictions and applies only
the current paid entitlement, without resetting traffic or granting registration-only
access. Queue/provider failures leave the release pending for retry. Unacknowledged
mutations receive `migration.detector_restriction_review` audit entries for operator
review; the panel cannot infer whether a timed-out upstream request took effect.
Previously applied timed IP blocks expire normally. Revoked credentials cannot be
recovered and continue to use the current credential lifecycle.

Before upgrading, keep a verified panel backup. After upgrading, remove the old
reporter cron entry `/etc/cron.d/tx-carpool-qps-detector` on the machines where it was
installed, then remove `/etc/tx-carpool-qps-detector.conf` and
`/usr/local/sbin/vansail-carpool-qps-detector-report`. Older renamed installations
may still use `/usr/local/sbin/tx-carpool-qps-detector-report`. The panel does not
SSH into nodes or remove operator-owned files. Old credentials are no longer
accepted and old upload routes return no detector service.

`internal/app/retired_detector_test.go` covers release ownership, manual restrictions
and retry behavior. `internal/platform/database/retired_detector_test.go` exercises
the embedded pre-078 schema upgrade, removed secrets, preserved history, canceled
jobs, ambiguous review and idempotency. These suites run in hosted CI.
