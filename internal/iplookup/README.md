# IP Lookup

## Responsibility and entry points

This module owns queued public-IP reputation checks, compact source-attributed
reports, owner-bound quotes, and provider normalization. `Service` accepts
canonical public IPv4/IPv6 addresses; database repositories own transactional
ledger/quota settlement and shared cache selection. `Worker` runs in the existing
provider-operation dispatcher. Each provider adapter enters its bounded
`upstreamqueue.Queue` before outbound HTTP; there is no second job lane.

Public contracts live in `types.go` and `report_types.go`; `config.go` validates
settings and hashes the frozen configuration. `worker.go` owns recovery and early
stopping. `verdict.go` folds transient evidence; `refusals.go` contains refusal
policy. `geo.go`, `provider_geo.go`, and `provider_sources.go` select coherent
location data and preserve enrichment lineage. `report_codec.go` is the sole
report persistence boundary.

- `address.go` canonicalizes public IPs; `service.go` signs quotes and exposes read-only cache previews.
- `registry.go` describes provider capabilities; `provider_http.go` executes bounded fixed-endpoint requests through the existing queue.
- `provider_parse.go`, `provider_insights.go`, and `provider_scamalytics.go` normalize documented responses without storing provider snapshots.
- `provider_test.go`, `provider_http_test.go`, and `worker_test.go` cover provider variants, queued transport and durable recovery in hosted CI.

## Data and recovery

`ProviderResult` is transient. Persist reports with `EncodeReport`, restore with
`DecodeReport`, and expose `PublicReport` to members. Never serialize individual
provider payloads as a report. The compact public projection contains contacted
database IDs/statuses, positive refusal items and evidence, one network type,
country/city/coordinate pair, ASN/name, selected-field sources, and four nullable
MaxMind Insights values: `ip_risk_snapshot`, `static_ip_score`, `user_count`, and
`user_type`. Missing values remain absent/nullable; explicit zeroes are preserved.
Region, ISP, provider-wide scores, and negative per-database flags are not stored.

While processing, `_checkpoint` retains only stage IDs/statuses/attempt markers,
aggregate risk coverage, majority votes, and the winning location/comparison
metadata. Before each provider call the attempted marker is durable. Completed
stages never replay; interrupted attempted stages become unavailable. Terminal
reports clear `_checkpoint`. `RecordProvider` folds a stage exactly once.

Legacy payloads are compacted on decoding/migration. Completed provider evidence
is folded into the checkpoint, preserving stage markers without new requests.
Old terminal status, verdict, reasons, policy version, and refund decision stay
frozen. Historical failed results remain ineligible for cache. Provider URLs,
credentials, and raw upstream errors never enter report snapshots.

## Ordering and verdict

The administrator's `providers` array is the check sequence; omitted known
providers are appended disabled. `geolocationOrder` is an independent complete
permutation, defaulting to IP2Location, IPAPI, MaxMind, Scamalytics, AbuseIPDB.
IPQS remains available for risk checks and network voting, outside location
selection. Pending work uses its frozen configuration and matches adapters by ID.

Positive per-IP abuse, datacenter, VPN, proxy, Tor, report count, or applicable
fraud-score evidence refuses the IP and stops subsequent providers. Scamalytics
low risk does not refuse solely for a positive score; medium/high/very-high do.
Explicit risk flags always refuse. MaxMind `user_count > 5` refuses and records
the actual count; 5 passes that check. Its other requested Insights values are
informational. Missing optional subscription fields remain unknown.

IPAPI network classification uses `is_datacenter`, `is_satellite`, and `is_mobile`
in that precedence. All three explicit false means residential; absent flags
stay unknown. Organization/ASN type never overrides these flags. One normalized
vote is counted per contacted provider. A type must exceed half the known votes;
ties and plurality without a strict majority stay unknown. Supporting source
IDs are retained. A suitable verdict requires complete configured risk coverage
and a residential/mobile majority; it does not promise service-specific access.

Contacted provider errors continue the sequence and are cached as partial,
inconclusive results. They never set `RefundRequired`, including an all-provider
outage or an outage before a later refusal. Actual infrastructure failures before
any provider attempt are distinguished using `ReportAttempted` and settled by
the database recovery path. Old refunded outcomes remain unchanged.

## Location selection

Each location candidate is one coherent source bundle; country, city, and
coordinates are never mixed across databases. Both coordinates must be finite
and within latitude/longitude bounds; zero is valid. City/coordinate candidates
take precedence over country-only fallback. Scamalytics enrichment sources are
named explicitly, such as `scamalytics:maxmind_geolite2`; AbuseIPDB geolocation
and network evidence is attributed to its documented IPinfo enrichment.

The following five bands are **application comparison policy**, not a claim that
different vendors' metrics are equivalent:

| Band | Explicit numeric confidence | Radius-only fallback |
| --- | --- | --- |
| VERY_HIGH | 95–100 | 0–10 km |
| HIGH | 80–94 | >10–50 km |
| MEDIUM | 60–79 | >50–200 km |
| LOW | 30–59 | >200–1,000 km |
| VERY_LOW | 0–29 | >1,000 km |

IPAPI native labels keep their band. MaxMind city confidence applies only to
city-bearing candidates; country confidence applies only to country-only data.
Use radius-derived bands when relevant explicit confidence is unavailable.
Known quality outranks missing quality. Within a band, smaller available radius
wins (missing radius sorts last), then administrator provider priority. This is
a total comparison so check order cannot change the winner. Explicit anycast
and IP2Location's capital fallback coordinates are VERY_LOW; an absent city is
never invented. The map must depict approximate IP location rather than a home.

## Cache, quote, and billing contracts

Quotes expire after two minutes and bind the owner, canonical requested IP,
frozen settings, and observed price/allowance. `QuoteResponse` adds response-only
`cachedReport` and `cacheMatch` (`none`, `exact`, `subnet`); signing/submission uses
only the embedded `Quote`. Cache previews do not create checks or consume quota.
Member receipts separately retain `requestedIP` and the original report's IP.

Reports have no automatic expiry. Exact-IP cache matches win; otherwise IPv4
can reuse a completed report in the same /24. IPv6 stays exact-only. Cache
previews are free and offer no refresh action; the existing priced refresh API
remains compatible. Configuration changes
do not rewrite cached policy/verdict. Fresh lookups consume TXB or an allowance
snapshotted at first successful purchase activation; polling never charges.

## Extension, references, and validation

Add providers through `Registry` and a queued `Provider` adapter, then extend
normalization, contracts, and localization. Keep vendor capability/omitted-field
semantics documented. HTTP responses are bounded, fixed-endpoint, and redacted.
Settings, vaulted secrets, combo quotas, and audit are committed atomically.

- [AbuseIPDB](https://docs.abuseipdb.com/): 90-day report window; no verbose comments.
- [Scamalytics v3](https://docs.scamalytics.com/ip-fraud-risk-api/v3/): EU `api12`, account username, tier placeholders, enrichment lineage.
- [IPAPI](https://ipapi.is/developers.html): keyed flags, accuracy bands, ASN organization, VPN/provider names.
- [MaxMind Insights](https://dev.maxmind.com/geoip/docs/web-services/responses/): omitted false anonymizer flags, city/country confidence, radius, exact Insights fields.
- [IPQS](https://www.ipqualityscore.com/documentation/proxy-detection-api/response-parameters): strictness 3 and unknown unavailable connection type.
- [IP2Location](https://www.ip2location.io/ip2location-documentation): usage codes, coordinates/capital fallback, ASN name, proxy provider, tier-specific fields.

Hosted fixtures cover HTTP queue/authentication boundaries, provider variants,
five/six-user refusal thresholds, confidence/radius boundaries, order-independent
location selection, strict majority, compact serialization, legacy recovery,
and continuing API errors without refunds. Local automated suites are prohibited.
Static vetting is separate from hosted test and frontend browser evidence.

`geo_test.go` checks quality boundaries and all check-order permutations;
`compact_report_test.go` checks strict majority, MaxMind count boundaries, and
legacy compaction; `config_order_test.go` checks administrator order validation;
`provider_provenance_test.go` checks enrichment lineage and refusal evidence.

## Live network details

`live_service.go` owns the `GET /api/v1/ip-lookup/details` workflow,
member admission (one active request, six starts/minute), cancellation, and a
60-second total deadline. `live_types.go` defines the public projection and queue
source identifiers; these sources are not reputation providers. No live result
is persisted, billed, or fed into suitability policy. Requested IPs are canonical
public addresses, even when reputation previews reuse a neighboring IP.

- `live_transport.go` owns bounded, fixed-origin, redacted HTTP through existing `upstreamqueue` workers.
- `live_bgp.go` selects the longest observed prefix, retains original paths/collector context, collapses visual prepends into edges, supports multiple origins, and bounds paths/nodes. RIPE Looking Glass falls back to RouteViews, then the explicitly dated RIPE BGP snapshot.
- `live_relationships.go` uses exact-pair CAIDA GraphQL queries (up to 200 pairs) and optional Cloudflare relationship fallback; missing relationships remain observed adjacency, never assumed transit.
- `live_names.go` caches only successful ASN names, with seven-day expiry and 4,096-entry LRU eviction. Keyed IPAPI bulk, BGPKIT, and bounded RIPE holder lookups fill missing names. Collector exchange scope is observation context, never an invented physical hop.
- `live_asn.go` selects registration from RIPE RIR `lod=2`, routing totals from RIPE/RouteViews, and dated relationship degrees from CAIDA/Cloudflare.
- `live_asn_prefetch.go` overlaps independent ASN reads through a lookup-local request memo.
- `live_requests.go` coalesces identical queued HTTP reads within a lookup and tracks transient failures.
- `live_pipeline.go` emits immutable section snapshots and progress over eight fixed logical phases; topology arrives before contextual enrichment finishes.
- `live_events.go` defines progress/section/complete/error stream events and optional refresh behavior. JSON clients keep the same response projection.
- `live_cache.go` shares concurrent lookups and stores eligible final projections for 60 seconds, bounded to 64 entries and 16 MiB. The cache key hashes canonical IP, configuration and live credentials; refresh bypasses completed cache, and transient failures prevent storage. A shared lookup is cancelled when its last subscriber leaves.
- `live_classification.go` batches explicit CAIDA clique membership with dataset dates; missing classifications remain nullable and never derive from degree counts.
- `progress.go` projects private durable checkpoints into check progress without exposing unfinished verdicts or changing persistence.
- `live_cache_test.go` covers expiry, credential invalidation, forced refresh, byte bounds, early streaming and failure exclusion in hosted CI.
- `live_ip.go` supplies keyed contextual IPAPI scores, distinct AbuseIPDB reported addresses in the announced block (30 days), exact decimal address capacity, and queued PTR lookups. Missing data remains nullable; block-plan restrictions never silently narrow the subnet.
- `live_traffic.go` requests Cloudflare Radar's BOT_CLASS, DEVICE_TYPE, and IP_VERSION summaries for the ASN over seven days; returned Other device values and actual date windows are preserved.
- `live_test.go` and `live_http_test.go` provide hosted regressions for prefix/origin/path preservation, name-only cache bounds, admission, queue enforcement, credentials, unknown/zero values, restrictions, and Cloudflare windows.

Cloudflare credentials use the existing vaulted settings map under
`cloudflare_radar`, independent of ordered risk-provider configuration. Blank
means keep; explicit clear removes it. No token or source payload reaches public
reports, audit events, or errors. Public adapters can return partial/unavailable
sections independently; external datasets retain their own observation dates.
`live_shodan.go` fetches InternetDB associated hostnames and observed open ports through the Shodan source queue. Expected no-information responses are empty, failures stay separate, invalid/out-of-range values are excluded, and no vulnerability/tag verdict is added. InternetDB is a weekly dataset without per-host observation timestamps. See https://internetdb.shodan.io/ for API and commercial-use licensing terms.
`live_shodan_test.go` covers sorted/deduplicated InternetDB hostnames and ports, unobserved addresses, provider outages, mismatched IPs, absent fields and invalid port ranges in hosted CI.
`live_integration_test.go` exercises the complete queued response projection with multiple origins, original prepends, source fallback, zero/unknown fields, live neighboring-IP requests, name-only cache behavior and credential-free public JSON in hosted CI.
Topology limits are 300 nodes, 1,500 edges and 1,000 original paths; omitted paths are disclosed. InternetDB retains at most 256 hostnames and 2,048 ports and marks larger observations partial.
- `live_cache_storage.go` validates cache eligibility and enforces TTL, entry and byte bounds.
- `live_shared_test.go` verifies shared subscriber cancellation and explicit dated CAIDA `cliqueMember` classification in hosted CI.
