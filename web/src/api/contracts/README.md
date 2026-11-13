# Frontend API contracts

Configuration deletion retains its existing DELETE operation and now declares
not-found and protected-use conflict responses alongside administrator access
failures. Generated request/response types mirror these transport outcomes.

Usage Breakdown requires `hourlySquads` and combined `positiveEntries`, while
retaining deprecated physical-only `positiveNodes`. Generated shapes protect the
typed rows; the API module additionally enforces exact conservation, combined
limits and separate node/squad identity uniqueness before rendering a snapshot.

Endpoint previews require derived metadata, expanded inbound choices and typed
field/client error arrays, with no credential/configuration output. Generated
`responseSchemas.json` and `responses.test.ts` reject missing facts and malformed
compatibility data while accepting future optional fields. The backend's typed
preview response and `endpoint_preview_test.go` enforce its six safe metadata
fields and exclude credentials or native configuration output.

- `responseSchemas.json` contains generated response shapes from OpenAPI. `responses.ts` lazily builds Zod validators for optional rendering snapshots, accepts new optional fields, and rejects malformed cached data before restoration. Live responses remain authoritative during rolling deploys. `responses.test.ts` covers additive changes and malformed snapshot shapes.

Ledger response validation accepts the server's active add-on, refund, affiliate,
raffle, IP lookup and Telegram deduction categories, along with retained retired
financial evidence. The contract regression rejects unknown ledger categories
without discarding known historical transactions.

Native node responses omit the retired traffic multiplier. The node write schema
accepts only editable identity, name, enablement, country/provider and configuration
fields. Retired multiplier and server-managed revisions, timestamps and agent
bindings are rejected; frontend editors no longer echo them. Historical catalog
and statistics accounting metadata remains a separate contract.

- `activity.ts` defines instant and raffle administration, quantized reward ranges, settled outcomes, and forecast responses.
- `commerce.ts` defines Emby and payment responses, including discovered crypto currency/network metadata and pending crypto orders.
- `admin.ts` defines database, statistics, onboarding, and restore responses.
- `affiliates.ts` defines member referral projections and versioned tier editor contracts.

Endpoint methods live in `../features.ts`; generated core schemas live in
`../generated.ts` and are regenerated from the application OpenAPI document.

- `activity.ts` defines `GroupBoostStatus` (verified state, nullable personal count and boost link); effective reward values come from the server.

`community.ts` contains coupon definitions, wallet grants, and redemption contracts.

Generated request contracts preserve required boolean moderation edits and a required bulk-extension duration. Affiliate discriminator mappings use the wire values `none`, `coupon`, `txb`, and `subscription_extension`. `../openapiContract.test.ts` checks these generated types during typecheck; `../openapiInputBoundaries.test.ts` retains their runtime input constraints.
