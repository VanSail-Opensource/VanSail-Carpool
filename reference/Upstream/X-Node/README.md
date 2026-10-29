# Pinned X-Node protocol

Source: https://github.com/VanSail-Opensource/X-Node/tree/292c7a261c24f0bf9ce3685be328dad3fda863d6
Signed agent release: v2.0.16. The panel Go protocol/native/compute dependency
uses that exact source revision. `compute-pool.md` is its protocol reference;
public Go message contracts are in the pinned `pkg/protocol` and `pkg/compute`.
Older agents remain supported; compute batches require `compute_pool_v1`.

`open_connections_v1` adds total open client flows alongside sampled traffic-active
connections. The agent offers `X-XNode-Open-Connections: v1`; the panel echoes
acceptance and checks the hello capability before using totals. Legacy telemetry
uses `users.total_connections`, compact telemetry uses user field `o`. Idle flows
count toward presence, while connection limits still use sampled `connections`.
Older panels receive no extension fields, including queued reconnect snapshots.
Older agents expose unknown totals; their idle observations cannot prove offline.

Quality run 40851848591 and signed release run 40919624398 completed successfully.
The downloaded SHA256SUMS signature was verified with the repository trust key;
the trusted version comment equals v2.0.16. All six manifest entries were checked.
Both agent binaries record this exact revision with vcs.modified=false.

- amd64 agent SHA-256: 99d37b1a3eb4dfa72203c22356e4b8ae85ddffec21939e0dd996456954c060d1
- arm64 agent SHA-256: c477163b6ed9d65c0aa0d27caa728497432ce99c66df22ba81b11c3d51e26f32

Artifact verification is separate from executing local tests. Hosted agent
regressions and real-host checks passed before the release tag was published.
