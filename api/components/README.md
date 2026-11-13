# Component shards

- `security-schemes.yaml` defines independent Mini App and browser sessions, their three request-signing headers, scoped API tokens and checkout-issued receipt capabilities.
- `parameters.yaml` contains reusable path and query parameters, the shared-route browser-session selector, and generic or hourly idempotency keys.
- `responses.yaml` contains reusable JSON error responses.
- `schemas/` contains the bounded schema shards and its definition index.

Internal component references point through `../openapi.yaml` so the root registry remains the single public contract namespace.
