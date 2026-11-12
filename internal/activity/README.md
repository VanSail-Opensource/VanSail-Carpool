# Activity domain

- `AUDIT.md` records the lucky-draw security review, fixed failure paths, regression coverage and operational limits.

- `activity.go` defines games, betting, and daily check-in types and validation.
- `reward.go` defines typed draw effects, validation, and persistence-facing payloads.
- `reward_payload.go` enforces one reward kind and one value source per configuration; fixed TXB values share the random-range limits. Stored configurations are revalidated before settlement sampling.
- `reward_payload_test.go` covers incompatible fields, malformed ranges, monetary bounds, and valid payloads for every supported reward kind.
- `draws.go` defines instant probabilities, raffle stock, outcomes, and activity history.
- `draw_random.go` validates quantized ranges and samples uniform, truncated Gaussian, and power-law distributions with the injected secure integer source.
- `draw_random_test.go` covers power-law coupon quantiles, endpoint reachability, distribution shape across reward precisions, signed/multiplier splits, fixed ranges and random-source failures.
- `RANDOM_DISTRIBUTIONS.md` defines distribution semantics, quantization and the correction to minimum-heavy power-law rewards. Changes affect future unresolved rewards; saved outcomes and awarded coupons retain their values.
- `raffles.go` defines group-entry receipts and settlement summaries; `service.go` exposes publication, entry, and settlement to HTTP and the durable worker.
- `service.go` validates and coordinates member and administrator activity workflows.
- `random.go` provides the cryptographically secure random source.
- `activity_test.go` verifies validation, idempotency, randomness, and service behavior.

- `group_boost.go` defines live group eligibility, the provider interface, fail-closed check-in gating, and exact half-up scaling in TXB hundredths. Bootstrap injects the queue-backed provider; message progress uses trusted Telegram sender metadata.
- `group_boost_test.go` covers boost multipliers, cent rounding, overflow, missing verification and settlement refusal.
- `group_messages.go` defines boosted message-reward configuration, validation and progress/result types, split from draw configuration.

Traffic-reset rewards cannot be configured or sampled. Their discriminator is
read-only historical evidence. The database adapter blocks old definitions from
new entry, cancels unresolved raffles with exact fee/reservation refunds and
queues Telegram announcement cleanup. Already committed results and entitlement
application jobs retain their original financial/accounting evidence.

Prize previews use the current probability/stock model. Unserialized retired `Weight` and `StockRemaining` fields that existed only for old test fixtures are removed; immutable saved draw snapshots and results are unchanged.
