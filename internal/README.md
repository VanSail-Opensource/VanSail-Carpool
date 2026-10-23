# Internal modules

Application code is private to this Go module and grouped into domain services, provider integrations, HTTP/application composition, and runtime platform packages.

`connections/` owns transient provider scans and signed member drop capabilities.

`purchaseops/` owns paid reset and first-term member refund commands and reconciliation.

`providerops/` owns provider-neutral receipts, items, replay metadata, and kind dispatch.

`statistics/` owns cached product metrics and scheduled X-Node host remark upkeep.

- `ARCHITECTURE.md` describes the dependency direction and ownership boundaries for internal packages.
