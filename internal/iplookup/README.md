# Queued IP lookup

`Service` signs owner-bound quotes; the database owns charge/quota settlement and receipt replay. `Worker` resumes paid checks through provider operations, shared request queues and durable checkpoints. `LiveService` supplies exact-address network context. APIs expose only normalized, bounded public projections.

Reputation adapters retain their registry, provider ordering and encrypted credentials. RPKI, BGP, ASN, PTR, Shodan InternetDB and transfer history remain available. All external requests use existing source queues. `enrichment.go` adds read-only live sections after quality stages and retains the legacy checkpoint JSON key for interrupted operations.

IP-source inventories, active port/protocol probes, neighbor checks and subnet cache sharing are retired. Existing stored reports and verdicts remain historical facts; new public projections omit retired scan fields. Exact-IP refreshes cannot replace neighboring evidence. The USDT memory soak resides in `internal/usdt`.

New reputation providers extend the registry and queued adapter contract. Network context extends typed live sections, event projection and their shared source queue. Do not add arbitrary backend calls or revive scan/background inventory execution.

Feature implementation and contracts: `address.go`, `background.go`, `background_run.go`, `cache_key.go`, `config.go`, `enrichment.go`, `geo.go`, `progress.go`, `provider_geo.go`, `provider_http.go`, `provider_insights.go`, `provider_parse.go`, `provider_scamalytics.go`, `provider_sources.go`, `rating.go`, `refusals.go`, `registry.go`, `report_codec.go`, `report_types.go`, `retry_policy.go`, `rpki_lookup.go`, `rpki_repair.go`, `rpki_types.go`, `service.go`, `shodan_verdict.go`, `types.go`, `verdict.go`, `warnings.go`, `worker.go`, `worker_config.go`, `worker_live.go`, `worker_lock.go`, `worker_provider_context.go`, `worker_retry.go`, `worker_rpki.go`.

Hosted regression coverage: `compact_report_test.go`, `config_order_test.go`, `geo_test.go`, `live_cache_test.go`, `live_graph_projection_test.go`, `live_http_test.go`, `live_integration_test.go`, `live_metadata_test.go`, `live_relationship_bulk_test.go`, `live_retry_test.go`, `live_shared_test.go`, `live_shodan_test.go`, `live_snapshot_test.go`, `live_test.go`, `live_tier1_test.go`, `live_transit_context_test.go`, `provider_http_test.go`, `provider_provenance_test.go`, `provider_test.go`, `rating_test.go`, `retry_policy_test.go`, `rpki_lookup_test.go`, `shodan_verdict_test.go`, `worker_config_test.go`, `worker_live_test.go`, `worker_retry_test.go`, `worker_rpki_test.go`, `worker_test.go`, `worker_warnings_test.go`.

Network context and streaming: `live_asn.go`, `live_asn_prefetch.go`, `live_bgp.go`, `live_cache.go`, `live_classification.go`, `live_compact.go`, `live_contacts.go`, `live_events.go`, `live_graph_projection.go`, `live_ip.go`, `live_names.go`, `live_pipeline.go`, `live_relationship_bulk.go`, `live_relationships.go`, `live_requests.go`, `live_retry.go`, `live_service.go`, `live_shodan.go`, `live_snapshot.go`, `live_tasks.go`, `live_tier1.go`, `live_traffic.go`, `live_transfer.go`, `live_transit_context.go`, `live_transport.go`, `live_types.go`, `live_websites.go`.

`retired_features_test.go` covers old settings/report projection, interrupted enrichment compatibility and progress that waits for network enrichment.

`run_lease.go` carries a fresh execution fence through background, provider and detached live contexts. Running-report persistence requires that owner and an unexpired database lease; heartbeat cancellation alone cannot authorize stale progress or settlement. `background_lease_test.go` verifies distinct owners across executions and fence propagation through reads, provider work, progress and completion.

Frozen report and live-snapshot repairs carry a transient `RepairRevision`
derived from the exact stored document. Each database write compares that
revision and immutable IP identity, returning the next revision without changing
cache ordering. Reputation progress advances its token after every committed
stage; losing repairs stop without overwriting newer sections or scheduling
another chain. The token is excluded from public and stored JSON.
`repair_test_helpers_test.go` supplies detached, revision-checking fixtures;
`repair_revision_test.go` covers competing commits and token privacy, while
`worker_retry_test.go` and `live_retry_test.go` retain policy/section recovery cases.
