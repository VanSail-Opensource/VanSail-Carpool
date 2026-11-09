# Pinned X-Node protocol

Source: https://github.com/VanSail-Opensource/X-Node/tree/bbe463662a0788b385183aa6ff12f87e3bec77b1
Signed agent release: [v2.0.23](https://github.com/VanSail-Opensource/X-Node/releases/tag/v2.0.23).
The panel protocol, native validator, compute and release-trust packages use this
exact verified source revision, represented by Go module pseudo-version
`v1.0.2-0.20261109022121-bbe463662a07`.

`protocol.md` and `hourly-account-limits.md` are copied from that revision.
`compute-pool.md` documents the
independently negotiated computation contract. Public Go types and validators
remain authoritative in the pinned `pkg/protocol`, `pkg/xrayconfig`, `pkg/compute`
and `pkg/releases` packages.
The complete public schemas and documentation are included in the signed
`x-node-api-docs.zip` release asset.

## Verification evidence

- [Quality run 42550599452](https://github.com/VanSail-Opensource/X-Node/actions/runs/42550599452): all six hosted gates succeeded, including race, schema, memory, vulnerability, both architecture builds and Debian nftables-on/off VMs.
- [Signed release run 42552560368](https://github.com/VanSail-Opensource/X-Node/actions/runs/42552560368): all three build, isolated signing and publication jobs succeeded.
- The signed manifest verified against the existing repository trust key with
  trusted version comment `version:v2.0.23`. All eight published assets were
  downloaded again; all six payload hashes, both agent/CLI ELF architectures and
  the firewall range contract/schema inside the documentation ZIP verified successfully.
- Both agent and both CLI binaries identify source revision
  `bbe463662a0788b385183aa6ff12f87e3bec77b1`, `vcs.modified=false`, Go 1.27.3,
  Linux and their
  expected amd64/arm64 architecture. The immutable annotated version tag peels
  to that same source commit.

| Agent | SHA-256 |
| --- | --- |
| amd64 | `9548fe38f881f8b9371f6e604800df4a2dabdba5d78a7f7040cd4c4ad8c8c8b8` |
| arm64 | `b885e51538855b301b74e10e4017f61b2cd5ab053c512a20e74a045781ce142d` |

Artifact signature, hash and metadata checks are separate from local automated
tests. Regression suites and benchmarks run only in hosted CI.

## Go dependency refresh

The native core is pinned to Xray v26.10.31, full commit
`a4a3dff5b20c2faae567b742c172c7b95e75c083`, using module version
`v1.260327.1-0.20261031132019-a4a3dff5b20c`. Its default MPTCP mitigation
does not replace host-kernel updates. Shared native validation also rejects
malformed XHTTP HTTP/2 field types before certificate preparation or listener
activation, following upstream alias, `extra` and download-setting precedence.
Unknown fields retain upstream handling; referenced node-local files still need
their existing preparation/validation environment.

The panel also selects pgx v5.12.0, `golang.org/x/net` v0.62.0,
`klauspost/compress` v1.20.3 and Brotli v1.2.8. Go remains 1.27.3.
Existing intentional pseudo-version selections, including `netipx` commit
`3075585bcbeb7a9f042662e543b03efe900018be`, YAML, QUIC and uTLS, are preserved.
Use explicit versions/commits rather than bulk `@latest`, which can select an
older tagged release over a reviewed pseudo-version. `go.mod` and `go.sum` are
authoritative; `go mod tidy` may remove stale unused sums without changing a
selected dependency. Hosted panel checks remain the integration acceptance gate.

The agent is available by exact tag but is deliberately not Latest. The compatible
panel must pass hosted validation, deploy and receive production acceptance before
the operator promotes this release for automatic discovery. Deployment and
production verification remain pending; updating the dependency adds no production
mutation. See the [security finding matrix](../../../docs/security-findings.md).

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
- `hourly_account_limits_v1`: shared member-ingress rate grants, durable funded
  byte credits and exact paid expiry. Historical mixed reports use independent
  `X-XNode-Hourly-Accounting: v1` receipt negotiation when live admission is
  unavailable. Missing grants pause hourly payload without closing ordinary
  accounts or discarding journals. See `hourly-account-limits.md` for units,
  replay, draining and recovery constraints.

The panel now implements atomic hourly history ingestion and gated hourly
commerce. Transport negotiation alone never enables sales or converts a customer.
Administrator policy and offers must enable sales, and every source/transit
participant must support the required capabilities before hourly authorization
can activate. Existing purchases and native identities retain their modes.

Ordinary commands, credentials and desired configuration retain durable
behavior. Periodic status-file writes are replaced by a permission-restricted
Unix status socket. Expensive host maintenance remains separate from the
master's sampling/report cadence. Manual/paid grants and member admission remain
independent of automatic leases. See the panel's
[lease operation and measurements](../../../docs/automatic-firewall-leases.md).
