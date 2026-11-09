# Frontend API contracts

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

- `responseSchemas.json` contains generated response shapes from OpenAPI. `responses.ts` lazily builds Zod validators for cached and refreshed reads, accepts new optional fields, and rejects malformed required data before it reaches Vue state. `responses.test.ts` covers additive changes and malformed successful responses.

Retired node traffic multiplier response properties remain constant 1. The
generated shapes and `responses.test.ts` reject obsolete nonconstant values in
current node/catalog responses. Their separate write contract accepts arbitrary
ignored legacy input, while the node editor omits that property entirely.

- `activity.ts` defines instant and raffle administration, quantized reward ranges, settled outcomes, and forecast responses.
- `commerce.ts` defines Emby and payment responses, including discovered crypto currency/network metadata and pending crypto orders.
- `admin.ts` defines database, statistics, onboarding, and restore responses.
- `affiliates.ts` defines member referral projections and versioned tier editor contracts.

Endpoint methods live in `../features.ts`; generated core schemas live in
`../generated.ts` and are regenerated from the application OpenAPI document.

- `activity.ts` defines `GroupBoostStatus` (verified state, nullable personal count and boost link); effective reward values come from the server.

`community.ts` contains coupon definitions, wallet grants, and redemption contracts.
