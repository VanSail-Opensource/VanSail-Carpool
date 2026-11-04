# Pinned X-Node protocol

Source: https://github.com/VanSail-Opensource/X-Node/tree/21a2158489fad0c4dd7858a088676eaa6215577e
Signed agent release: [v2.0.19](https://github.com/VanSail-Opensource/X-Node/releases/tag/v2.0.19).
The panel protocol, native validator, compute and release-trust packages use this
exact verified source revision, represented by Go module pseudo-version
`v1.0.2-0.20261104005232-21a2158489fa`.

`protocol.md` is copied from that revision. `compute-pool.md` documents the
independently negotiated computation contract. Public Go types and validators
remain authoritative in the pinned `pkg/protocol`, `pkg/native` and `pkg/compute`.
The complete public schemas and documentation are included in the signed
`x-node-api-docs.zip` release asset.

## Verification evidence

- [Quality run 41750210539](https://github.com/VanSail-Opensource/X-Node/actions/runs/41750210539): successful hosted race, schema, memory, vulnerability, both architecture builds and Debian nftables-on/off VM gates.
- [Signed release run 41752453684](https://github.com/VanSail-Opensource/X-Node/actions/runs/41752453684): successful publication.
- The downloaded SHA256SUMS signature verified against the previously pinned
  repository trust key and its trusted version comment matched v2.0.19.
- All six manifest entries matched their downloaded SHA-256 values. Both agent
  and both CLI binaries identify source revision
  `21a2158489fad0c4dd7858a088676eaa6215577e`, `vcs.modified=false`, Linux and their
  expected amd64/arm64 ELF architecture. Read-only amd64 `-version` output also
  reported v2.0.19, commit 21a2158, Xray v26.10.29 and all four new capabilities.

| Agent | SHA-256 |
| --- | --- |
| amd64 | `a8118e8576cd3487b663e76199bb65ac6003210f14189e7b7ca9b14a6a976e1a` |
| arm64 | `189b18aab105af738c8853ea4a4c5d69fadbd5908c0695cd94f174e8fd5e5cb1` |

Artifact signature, hash and metadata checks are separate from local automated
tests. Regression suites and benchmarks run only in hosted CI.

## Coordinated capabilities

Upgrade the panel before agents. New boot-scoped volatile traffic requires the
updated master; existing durable journals retain their identities and drain
before new streams. The panel continues decoding deployed legacy report formats.

- `report_zstd_l6_v1`: negotiated independent Zstandard level-6 report frames,
  reused single-worker encoding, bounded decoding and no double compression.
- `traffic_stream_v1`: boot identity, immutable sequence/digest handling, bounded
  volatile buffers and deterministic replay/gap/conflict outcomes. Accounting
  commits before ACK and settlement uses surviving known usage.
- `activity_observations_v1`: actual sample and last-activity timestamps, bounded
  two-hour IP history, exact counters and mandatory master-controlled sampling.
- `volatile_firewall_leases_v1`: private batched exact-address exemptions with
  absolute deadlines, epoch/generation replay handling and local kernel expiry.

Ordinary commands, credentials and desired configuration retain durable
behavior. Periodic status-file writes are replaced by a permission-restricted
Unix status socket. Expensive host maintenance remains separate from the
master's sampling/report cadence. Manual/paid grants and member admission remain
independent of automatic leases. See the panel's
[lease operation and measurements](../../../docs/automatic-firewall-leases.md).
