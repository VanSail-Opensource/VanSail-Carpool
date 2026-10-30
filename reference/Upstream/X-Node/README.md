# Pinned X-Node protocol

Source: https://github.com/VanSail-Opensource/X-Node/tree/bdefc59f3479290ec23a744d6530524d64db91ec
Signed agent release: v2.0.17. The panel Go protocol/native/compute dependency
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

Quality run 41028256572 and signed release run 41030338545 completed successfully.
The downloaded SHA256SUMS signature was verified with the repository trust key;
the trusted version comment equals v2.0.17. All six manifest entries were checked.
Both agent binaries record this exact revision with vcs.modified=false.

- amd64 agent SHA-256: 5483845b6534aa37cd5c76b781fba506a4a8d6369208d7b0968c97fb75f4505d
- arm64 agent SHA-256: 5fe55d55ba1b4a47c0c71b9689e28aece5e3ebfb6668da68c00b1b8b8c0fbff2

Artifact verification is separate from executing local tests. Hosted agent
regressions and real-host checks passed before the release tag was published.

The signed release moves status/report encoding to a bounded coalescing worker,
uses narrow state metadata, drains sealed traffic replay after matching ACKs,
and reconnects on ordered inbound saturation. Duplicate ACKs do not write disk.
New reporting defaults target two seconds; explicit saved panel intervals and
legacy omitted intervals remain effective. Expensive host scans use a separate
ten-second cadence. Existing wire envelopes and negotiated capabilities remain
compatible with older agents and masters.

Hosted metadata comparison (1000 synthetic users, 100 iterations): full snapshot
904669 ns/op and 948964 B/op; metadata 24.83 ns/op and zero allocation. These are
isolated benchmark results, not a production performance measurement.
