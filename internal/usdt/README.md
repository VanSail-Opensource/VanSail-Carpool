# Built-in USDT and USDC collection

New token orders use the shared billing USD rate. Legacy encrypted rate fields
are ignored and removed on save; collectors and issued quotes remain independent
of rate availability. Stored order rate snapshots never change.

`Service.Start` runs inside the existing application lifecycle. `Methods`
and `Expiry` supply billing with local configuration/readiness; `Create` reserves
an address/amount fingerprint through SQLite. Every external call enters
`upstreamqueue` through `Executor`, including probes, reconnects and WS commands.
Inbound WS notifications only wake/coalesce scans; they never settle an order.

Only pinned native mainnet identities are accepted. `registry.go` pins USDC from
[Circle](https://developers.circle.com/stablecoins/usdc-contract-addresses), read 2026-10-18,
on Ethereum, Polygon, Arbitrum, X Layer, Plasma, Solana and Aptos, all with six decimals.
Queued readiness probes verify mainnet and expected precision. Bridged identities and
USDC on TRON/BSC/TON are excluded. Token amounts use integer
atomic units; quotes use three decimal places and at most 99 increments of 0.001.
Fingerprints survive paid-order cleanup and are never reused. Capacity exhaustion
requires another configured address. Expired/cancelled unpaid snapshots and review
evidence remain available for late-transfer resolution. Transfers are consumed in
the existing ledger transaction, including notifications and operation recovery.
Exact amount matches take priority. Auto-credit accepts only an underpayment of at
most 0.001 tokens when it identifies one quote for the same token, network and address;
ambiguous matches stay in review. Administrators select verified matching evidence inside the selected payment credit
dialog. Overpayments and ambiguous shortfalls cannot settle. Review dismissal records
actor, reason, time and audit evidence; dismissed and credited evidence is immutable.

Each network has an authoritative `currencies` list containing `USDT`, `USDC`,
both where registered, or an explicit empty list. Missing/null lists inherit legacy
`mode: usdt | usdc | both` (missing mode defaults to USDT) and become explicit on save. Endpoints, addresses,
expiry and the shared FIFO concurrency cap remain unchanged. Scanners and
readiness use token rails; cursors persist network plus native contract identity.
Issued quotes keep their token watchers through network/token switch changes. Transfer IDs remain
network/transaction/event hashes, and permanent quote fingerprints are untouched.

The whole configuration uses the existing secret vault under `billing.usdt.config`.
Administrative projections return only URL hostnames and configured flags. Blank
URL/key writes retain the saved value. The dedicated save validates outstanding
quote coverage; generic setting writes cannot bypass it. No private keys, payouts,
exchange-rate APIs or merchant callbacks are implemented.

All RPC, indexer, probe, reconnect and WS command work enters one FIFO queue with a
single shared concurrent-request limit. Failed requests do not create persisted
cooldowns or consume a daily or minute budget. The queue retains a bounded backlog;
provider errors are retried by the normal scanner failover loop.

HTTP responses decode from a bounded reader into typed destinations inside the
shared queue slot; RPC/GraphQL envelopes borrow their result bytes rather than
retaining a second raw payload. HTTP documents and WS messages share a 2 MiB
limit. Reconnects join the retired WS reader before allocating its replacement.
Aptos retains only the current transaction version while processing ordered
activities, releasing its event array before fetching the next version. EVM
receipt groups refer to log positions; verified groups and processed logs are
discarded during the batch. Durable quote fingerprints, transfer evidence and
scan cursors keep their existing retention and settlement rules.

EVM uses finalized heads (BSC native finality), filtered USDT receiving logs and
successful receipts. Receipt responses are verified one transaction at a time
and discarded immediately; the batch retains only verified transaction hashes.
TRON requires a TronGrid-compatible root; Aptos requires a
node root plus GraphQL indexer; TON requires a Toncenter-compatible `/api/v3` base.
Solana scans all discovered USDT token accounts and their deterministic ATA, using
finalized transaction instructions rather than net balance changes. Non-EVM indexed
history resumes stable cursors and uses an overlapping 30-minute time window for
TRON/TON index visibility. Historical Aptos store verification needs retained
ledger versions. Server history/proof failures preserve cursors and close readiness.

Extend collection by adding a pinned registry entry, adapter validation/normalization,
wire contracts, localization and hosted fixtures. Never accept a token by symbol
alone or advance a cursor past a failed page. The supported deployment is one Go
process per SQLite database. Local automated suites are prohibited; tests below
run in hosted CI.

## Files

- `address.go` ? Canonical network address validation, checksums and base58 encoding.
- `amount.go` ? Exact integer token arithmetic and fixed-decimal formatting.
- `amount_test.go` ? Hosted fixtures for amount behavior and failure paths.
- `aptos.go` ? Filtered GraphQL activities cross-checked with node transactions and historical stores.
- `aptos_proof.go` ? Committed deposit-event identity, quantities and timestamps.
- `aptos_test.go` ? Hosted fixtures for aptos behavior and failure paths.
- `config.go` ? Configuration validation and secret-preserving masked projections.
- `config_test.go` ? Hosted fixtures for config behavior and failure paths.
- `currencies.go` ? Authoritative per-network USDT/USDC switches with legacy inheritance.
- `currencies_test.go` ? Hosted fixtures for switch authority and issued-quote watcher continuity.
- `evm.go` ? Finalized filtered EVM log ranges and canonical cursor checks.
- `evm_proof.go` ? Successful receipt/log consistency and exact hexadecimal quantities.
- `evm_test.go` ? Hosted fixtures for evm behavior and failure paths.
- `executor.go` ? One shared upstreamqueue with a global concurrent-request limit.
- `lifecycle.go` ? Context-owned reconciliation, status and durable receipt recovery.
- `monitor.go` ? Shared network scans, bounded catch-up, endpoint priority and WS coalescing.
- `pricing.go` ? Local payment availability and immutable quote creation.
- `probe.go` ? Queued administrative mainnet and token-capability probes.
- `registry.go` ? Pinned mainnet USDT contracts, chain identities, decimals and disabled defaults.
- `response.go` ? Bounded document decoding and typed RPC/GraphQL envelope fields.
- `response_test.go` ? Hosted response bounds, envelope validation and decoding allocation benchmarks.
- `scanner.go` ? Adapter dispatch, normalized receiving-event evidence and resumable time windows.
- `service.go` ? Encrypted settings, issued-quote recovery, review and safe error identifiers.
- `solana.go` ? Finalized token-account history with signature cursors.
- `solana_accounts.go` ? SPL token account discovery and associated-account derivation.
- `solana_proof.go` ? Successful outer and CPI token transfer instruction evidence.
- `solana_test.go` ? Hosted fixtures for solana behavior and failure paths.
- `test_helpers_test.go` ? Hosted fixtures for test helpers behavior and failure paths.
- `ton.go` ? Toncenter jetton history, approved wallets and complete traces.
- `ton_proof.go` ? Receiving jetton wallet execution, message linkage, BOC amount and masterchain inclusion.
- `ton_test.go` ? Hosted fixtures for ton behavior and failure paths.
- `transport.go` ? Queued HTTP / JSON-RPC transport, bounded responses and redacted failures.
- `transport_test.go` ? Hosted fixtures for transport behavior and failure paths.
- `tron.go` ? Confirmed TronGrid account history with stable pagination parameters.
- `tron_proof.go` ? Solidified TRC20 receiving logs and successful execution receipts.
- `tron_test.go` ? Hosted fixtures for tron behavior and failure paths.
- `types.go` ? Public configuration, quote, transfer, status, repository and settings contracts.
- `websocket.go` ? Queued connection and subscription commands with context-owned inbound readers.

## Allocation ownership

`page.go` enforces limits before appending oversized response arrays: 100
transfer/signature rows, 128 token accounts, 200 Tron history rows and 2,000 EVM
logs. Shared JSON guards bound depth, array cardinality and graph nodes before
typed expansion, retaining the 2 MiB HTTP/WS document limit. Cancellation joins
started executor callbacks before output is released or replacement work starts.
`solana_cursor.go` compacts only owners outside the complete watched recipient
set; closed accounts and unpaid late-payment recipients remain monitored.
`memory_test.go` covers graph/page limits, cursor retention and callback joining.
The single shared concurrency setting, settlement and durable evidence retention
are unchanged. Lifecycle logs memory and goroutines once per minute.

`tokens_test.go` supplies hosted pinned-registry, legacy-default and token-readiness isolation regressions.

`token_probe_test.go` verifies that queued native USDC probes use each pinned EVM contract and reject unexpected precision.
