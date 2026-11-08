# Pinned X-Node protocol

Source: https://github.com/VanSail-Opensource/X-Node/tree/32b7e560b7f9e572f73a2d5bf8734d4335925437
Signed agent release: [v2.0.20](https://github.com/VanSail-Opensource/X-Node/releases/tag/v2.0.20).
The panel protocol, native validator, compute and release-trust packages use this
exact verified source revision, represented by Go module pseudo-version
`v1.0.2-0.20261108134236-32b7e560b7f9`.

`protocol.md` is copied from that revision. `compute-pool.md` documents the
independently negotiated computation contract. Public Go types and validators
remain authoritative in the pinned `pkg/protocol`, `pkg/xrayconfig`, `pkg/compute`
and `pkg/releases` packages.
The complete public schemas and documentation are included in the signed
`x-node-api-docs.zip` release asset.

## Verification evidence

- [Quality run 42486352821](https://github.com/VanSail-Opensource/X-Node/actions/runs/42486352821): successful hosted race, schema, memory, vulnerability, both architecture builds and Debian nftables-on/off VM gates.
- [Signed release run 42487713242](https://github.com/VanSail-Opensource/X-Node/actions/runs/42487713242): successful build, isolated signing and publication.
- The signed manifest verified against the existing repository trust key with
  trusted version comment `version:v2.0.20`. All six payload hashes and both
  agent/CLI ELF architectures were verified again after draft asset upload.
- Both agent and both CLI binaries identify source revision
  `32b7e560b7f9e572f73a2d5bf8734d4335925437`, `vcs.modified=false`, Linux and their
  expected amd64/arm64 architecture. The immutable annotated version tag peels
  to that same source commit.

| Agent | SHA-256 |
| --- | --- |
| amd64 | `b5dcf81f1fbbc83dec21d36559f548bd8d5a5198857b4359bc645b3ef54ba882` |
| arm64 | `7da25e2c576dabdd15f0af8c550b94c45bd96f1a8867fce2062a66f61e41222b` |

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

Ordinary commands, credentials and desired configuration retain durable
behavior. Periodic status-file writes are replaced by a permission-restricted
Unix status socket. Expensive host maintenance remains separate from the
master's sampling/report cadence. Manual/paid grants and member admission remain
independent of automatic leases. See the panel's
[lease operation and measurements](../../../docs/automatic-firewall-leases.md).
