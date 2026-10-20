# Incremental Xray configuration delivery

## Compatible baseline

The panel pins X-Node source `e938bae9fc2905eb8752dbb30c008dc3428d8ba3`, released
as [v2.0.12](https://github.com/VanSail-Opensource/X-Node/releases/tag/v2.0.12).
Its native validator and the panel's direct core dependency both use
`v1.260327.1-0.20261017152356-92ccc939aaf4` (Xray v26.10.17). This preserves the
core upgrade merged through X-Node PR #1. CLI and handshake versions come from
the embedded core rather than a separate hardcoded version.

## Responsibilities and data flow

The existing master queue sends authoritative state to the agent's serialized
activation worker. Registry/utility/firewall consumers bypass native compilation
when their native inputs are unchanged. Native account, listener, outbound and
routing protobufs are reused independently; supported updates retain the core
instance. Listener epochs contain effective listener/certificate material and
exclude unrelated global routing. Tail-only routing appends reuse old matchers;
reordered/replaced tables use the public native replacement API.

The managed outbound wrapper retains the public selector interface for balancers
and serializes upstream selector-cache access with handler changes. Replacements
fence and drain affected dispatches. Cross-node routing keeps existing durable
revoke/apply/enable barriers and exact accounting. Certificate/asset rechecks
survive superseding control updates until activation succeeds.

The panel's transactional revision remains a coherence fence. Its per-node
effective fingerprints retain unchanged wire projections, and bounded memory-only
validation contexts reuse native sections/proofs. Invalidations still recheck
authorization, time boundaries, resolved routing dependencies and original local
asset declarations. Synthetic certificate/asset analysis never becomes serving
credentials or substitutes for agent validation. Metadata edits retain encrypted
documents. No database schema, public REST contract or frontend changes are needed.

Explicit reloads and unsupported static core/default-outbound/dependency changes
retain validated reconstruction. Native preparation and protobuf copying happen
before listener shutdown; diagnostics measure the remaining gap in microseconds.

## Agent verification

- [Quality](https://github.com/VanSail-Opensource/X-Node/actions/runs/39492358250)
  passed race/real-stream regressions, schemas, static analysis, vulnerability
  checks, both architecture builds, and Debian with nftables ownership on/off.
- [Signed release](https://github.com/VanSail-Opensource/X-Node/actions/runs/39494328163)
  passed for the same source commit.
- Downloaded agent and CLI files for both architectures were checked against the
  production, version-bound Minisign manifest. ELF64 machine types were checked,
  and both agent build records contain the expected core dependency and source
  commit with `vcs.modified=false`.
- No local automated suites or production fleet commands were run. Panel
  regressions/container acceptance run through the main-branch Container workflow.

Verified agent SHA-256 values:

| Architecture | SHA-256 |
| --- | --- |
| amd64 | `559d2937a5c100d392f9a6497692e44fb962566968775a52756cf9a1c9df45b6` |
| arm64 | `f3ab427befd229596a12e0f458edf3e320e009773e1573ef93cc6cc84ec99295` |

## Hosted comparison

The Quality `reload-benchmark` artifact uses the same workload for both paths:

| Scenario | Cold / forced | Cached / live |
| --- | --- | --- |
| Preparation, 1,000 users | 11.132441 ms/op | 8.027843 ms/op |
| Allocated preparation bytes, 1,000 users | 5,906,074 B/op | 3,815,387 B/op |
| Core reconstructions across 20 rule edits | 20 | 0 |
| Last measured listener gap | 1,852 microseconds | 0 microseconds |

These are hosted fixture measurements, not a production latency guarantee.
Real TCP, UDP/XUDP, mux, credential rotation and accounting regressions also run
in the agent Quality workflow; managed routing barriers and cache invalidation
remain covered by the panel's hosted regressions.
