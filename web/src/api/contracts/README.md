# Frontend API contracts

- `responseSchemas.json` contains generated response shapes from OpenAPI. `responses.ts` lazily builds Zod validators for cached and refreshed reads, accepts new optional fields, and rejects malformed required data before it reaches Vue state. `responses.test.ts` covers additive changes and malformed successful responses.

- `activity.ts` defines instant and raffle administration, quantized reward ranges, settled outcomes, and forecast responses.
- `commerce.ts` defines Emby and payment responses, including discovered crypto currency/network metadata and pending crypto orders.
- `admin.ts` defines database, statistics, onboarding, and restore responses.
- `affiliates.ts` defines member referral projections and versioned tier editor contracts.
- `compensation.ts` defines generated aliases for outage configuration, events, pages, and review writes.

Endpoint methods live in `../features.ts`; generated core schemas live in
`../generated.ts` and are regenerated from the application OpenAPI document.

- `activity.ts` defines `GroupBoostStatus` (verified state, nullable personal count and boost link); effective reward values come from the server.

`community.ts` contains coupon definitions, wallet grants, and redemption contracts.
