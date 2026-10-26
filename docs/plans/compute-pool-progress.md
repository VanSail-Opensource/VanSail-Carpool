# Compute pool and pricing refinement delivery

## Baseline and sequence

The initial dirty pricing implementation is backed up under the local temporary folder `vansail-compute-baseline-20261026-195019`. Intended draft pricing work is included; unrelated Python caches remain excluded. Production inspection has been read-only. No local automated suites have run.

1. Retirement implemented and audited: expiry, period closure, refunds and successors use the purchase/accounting lifecycle. SQLite 106/PostgreSQL 18 cancel unsettled credits without wallet changes and archive settled audit facts. Operational services, schema, endpoints, UI, copy and active documentation are removed. Historical migrations remain immutable.
2. PAYG repair implemented and audited: incomplete identities and missing encrypted subscription credentials recover without Combo/funding prerequisites. New accounts immediately participate in synchronization; actual admission and settlement guards remain.
3. Shared versioned kernels, negotiated transport, independent queues, pressure hysteresis, bounded batches/zstd/chunks, first-claim fencing, leases and durable acceptance implemented. Agent quality run 40366302761 and signed release 40369427102 passed. v2.0.15 signature and all six artifact hashes were verified at revision c310c8f; the panel pin uses that revision without a local Go workspace.
4. Pricing and forecasts implemented: queued fitting/quantiles/candidates, minute PAYG recalculation, wallet-independent models/scheduling, scoped coupon economics, terminal unchanged decisions, dual-decision queued admission, atomic renewal disablement, prorated live squad coupons and original prepaid basis preservation. Personal forecasts distinguish observed coverage, intervals and measured calibration.
5. Interfaces implemented and inspected: shadcn-vue/Unovis chart composition, prediction flip card and PAYG-only Home, expanded admin/user analysis with explicit pagination, compact Live price copy, PAYG catalog states, owner host names/{price}, typed APIs and shared subscription metadata.

## Verification and remaining delivery gates

Local `go build`, `go vet`, API generation, frontend typecheck/build, lint, structure/i18n and diff checks passed during development. Windows-only CRLF formatter reports were separated from semantic formatting differences; the sole semantic difference was corrected. Browser evidence is in [compute-pool-browser-validation.md](compute-pool-browser-validation.md).

Hosted regressions are authored for SQLite/PostgreSQL retirement, exact finances, PAYG recovery, admission/coupons, compute races/stale/disconnect/malformed uploads and storage. The existing PostgreSQL isolated forward-upgrade rehearsal now seeds unsettled/settled credit evidence at Goose 17 and verifies balances, ledger/purchases, archived evidence and suppressed jobs after upgrade.

Panel implementation 51e69f8c is pushed to main, preserving upstream subscription fix 632401e4. Container run 40411286611 passed formatting, vet/security, CPU and memory gates. Its first regressions exposed stale frontend fixtures, an obsolete statistics error expectation, missing explicit DOUBLE PRECISION/SMALLINT aliases in the PostgreSQL catalog verifier, and a legacy endpoint-ID suffix reappearing when host names reset. These are corrected for the replacement run; stable routing tags remain unchanged. Other hosted regressions and final publication remain pending; overall delivery is not complete until the exact-head gates pass.
