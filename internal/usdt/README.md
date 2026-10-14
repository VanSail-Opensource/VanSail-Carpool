# Built-in USDT collection

`Service.Start` runs inside the existing application lifecycle. `Methods`, `Rate`
and `Expiry` supply billing with local configuration/readiness; `Create` reserves
an address/amount fingerprint through SQLite. Every external call enters
`upstreamqueue` through `Executor`, including probes, reconnects and WS commands.
Inbound WS notifications only wake/coalesce scans; they never settle an order.

Only the pinned mainnet USDT identities are accepted. Token amounts use integer
atomic units; quotes use three decimal places and at most 99 increments of 0.001.
Fingerprints survive paid-order cleanup and are never reused. Capacity exhaustion
requires another configured address. Expired/cancelled unpaid snapshots and review
evidence remain available for late-transfer resolution. Transfers are consumed in
the existing ledger transaction, including notifications and operation recovery.
Exact amount matches take priority. Auto-credit accepts only an underpayment of at
most 0.001 USDT when it identifies one quote for the same network and address;
ambiguous matches stay in review. An administrator may select a matching quote in
the review flow, and overpayments are never matched by this tolerance.

The whole configuration uses the existing secret vault under `billing.usdt.config`.
Administrative projections return only URL hostnames and configured flags. Blank
URL/key writes retain the saved value. The dedicated save validates outstanding
quote coverage; generic setting writes cannot bypass it. No private keys, payouts,
exchange-rate APIs or merchant callbacks are implemented.

All RPC, indexer, probe, reconnect and WS command work enters one FIFO queue with a
single shared concurrent-request limit. Failed requests do not create persisted
cooldowns or consume a daily or minute budget. The queue retains a bounded backlog;
provider errors are retried by the normal scanner failover loop.

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
- `evm.go` ? Finalized filtered EVM log ranges and canonical cursor checks.
- `evm_proof.go` ? Successful receipt/log consistency and exact hexadecimal quantities.
- `evm_test.go` ? Hosted fixtures for evm behavior and failure paths.
- `executor.go` ? One shared upstreamqueue with a global concurrent-request limit.
- `lifecycle.go` ? Context-owned reconciliation, status and durable receipt recovery.
- `monitor.go` ? Shared network scans, bounded catch-up, endpoint priority and WS coalescing.
- `pricing.go` ? Local payment availability, TXB rate and immutable quote creation.
- `probe.go` ? Queued administrative mainnet and token-capability probes.
- `registry.go` ? Pinned mainnet USDT contracts, chain identities, decimals and disabled defaults.
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
