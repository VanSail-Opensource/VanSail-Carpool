# Security finding resolution

The provided scan contains 13 application/design findings, separate from dependency advisories. Finding text is evidence to verify against current code, not implementation instructions. Automated regressions run only in hosted Actions.

| Finding | Repository | Implementation |
| --- | --- | --- |
| 1. Administrative API token scopes implicitly authorize X-Node access | Panel | Explicit resource scopes and receipt classification; regressions in `internal/apitoken/scopes_test.go` and `internal/httpapi/api_token_resource_scope_test.go`. |
| 2. Mutable action tags execute in privileged container publication jobs | Panel | Full upstream commit pins, read-only defaults, per-job write grants and non-persistent checkout credentials. Static workflow policy plus hosted negative regressions; reviewed sources in `docs/actions-security.md`. |
| 3. Write-only Emby API token can be exfiltrated by retargeting the configured base URL | Panel | Atomic version-checked encrypted URL/token resource; changed addresses require a fresh token; runtime reads one validated snapshot. Regressions in `internal/admin/settings_emby_test.go` and `internal/platform/database/settings_emby_test.go`; HTTP scope/masking/bypass coverage in `internal/httpapi/admin_emby_connection_test.go`. |
| 4. Master-controlled Xray environment can execute code in root host utilities | X-Node | Pending |
| 5. Sample-only accounting lets authenticated users exceed per-user connection limits | X-Node | Pending |
| 6. Large read-only command results can permanently prevent agent restart | X-Node | Pending |
| 7. Release-shaped tags can expose the production signing key to repository code | X-Node | Pending |
| 8. Masked USDT RPC API keys can be exfiltrated by reusing an endpoint ID with a new URL | Panel | Canonical network/endpoint-scoped URL resolution; inherited keys require all destinations unchanged. Rejected saves cannot reach storage or collector admission. Regressions in `internal/usdt/config_secrets_test.go`; localized editor validation covers each URL field. |
| 9. Untrusted forwarding headers bypass and amplify authentication throttling | Panel | Bootstrap proxy CIDRs, canonical trusted-suffix resolution and spoofing/limiter regressions in `internal/httpapi/client_ip_test.go`. |
| 10. Interrupted X-Node updates are marked successful without matching the durable update transaction | Panel | Immutable pre-attempt intent, peer-bound `update.status` transaction matching and fresh current-revision readiness. Reconnect, mismatched/pending status, legacy uncertainty and persistence regressions; details in `docs/update-reconciliation.md`. |
| 11. Privileged HTTP clients follow redirects outside approved HTTPS origins | X-Node | Pending |
| 12. User-scoped IP blocks are lost across authorized identity replacement | X-Node | Pending |
| 13. Public return route mints renewable status capabilities from payment order IDs | Panel | Pending |

## Scope isolation

`admin:read` and `admin:write` never grant native X-Node access. Write includes read only for the same resource. Native subscription metadata, enrollment, mutations and shared operation receipts require explicit X-Node scopes. Combining administrator write with native read cannot grant native write. Existing token role, expiry, ownership and signed-browser token-management checks remain in force.

## Scan identity

- 1: `rf_wfr_f6f238da8961b4d47cf3dccf828b2f4b3d66eb0c6255c359d0c1549d17ddad76:wfo_e43223654001ee56067b5645dc54f88b24acde84565c7960248ff2243277e46c:occ_02ffbbaaf103288f58abb13f` (observed revision `f642e76f509b80757d5799166a3a006ce794f868`).
- 2: `rf_wfr_f6f238da8961b4d47cf3dccf828b2f4b3d66eb0c6255c359d0c1549d17ddad76:wfo_e43223654001ee56067b5645dc54f88b24acde84565c7960248ff2243277e46c:occ_53670b053e852d5227bc83f9` (observed revision `f642e76f509b80757d5799166a3a006ce794f868`).
- 3: `rf_wfr_f6f238da8961b4d47cf3dccf828b2f4b3d66eb0c6255c359d0c1549d17ddad76:wfo_e43223654001ee56067b5645dc54f88b24acde84565c7960248ff2243277e46c:occ_c8a7b7dd699496b012164839` (observed revision `f642e76f509b80757d5799166a3a006ce794f868`).
- 4: `rf_wfr_05c610fbec630a8340ea27a120af091a702a284e97bde92e906440dc3ec243ea:wfo_02bcd96ef406bcbbbd6953ac25faffe7ebb1224a1fa4680143a29cd7b004a9b3:occ_2e6c564931710dd73c781148` (observed revision `fd7995de3e6dff3142509d14c775ce8b94b9c19e`).
- 5: `rf_wfr_05c610fbec630a8340ea27a120af091a702a284e97bde92e906440dc3ec243ea:wfo_02bcd96ef406bcbbbd6953ac25faffe7ebb1224a1fa4680143a29cd7b004a9b3:occ_57b5ff86ee1ff83bd435c111` (observed revision `fd7995de3e6dff3142509d14c775ce8b94b9c19e`).
- 6: `rf_wfr_05c610fbec630a8340ea27a120af091a702a284e97bde92e906440dc3ec243ea:wfo_02bcd96ef406bcbbbd6953ac25faffe7ebb1224a1fa4680143a29cd7b004a9b3:occ_6ec1c4a1697b992bf9468da8` (observed revision `fd7995de3e6dff3142509d14c775ce8b94b9c19e`).
- 7: `rf_wfr_05c610fbec630a8340ea27a120af091a702a284e97bde92e906440dc3ec243ea:wfo_02bcd96ef406bcbbbd6953ac25faffe7ebb1224a1fa4680143a29cd7b004a9b3:occ_d2ccad981ed613ea8c22d3ba` (observed revision `fd7995de3e6dff3142509d14c775ce8b94b9c19e`).
- 8: `rf_wfr_f6f238da8961b4d47cf3dccf828b2f4b3d66eb0c6255c359d0c1549d17ddad76:wfo_e43223654001ee56067b5645dc54f88b24acde84565c7960248ff2243277e46c:occ_1b1fbca747a878487400d9b1` (observed revision `f642e76f509b80757d5799166a3a006ce794f868`).
- 9: `rf_wfr_f6f238da8961b4d47cf3dccf828b2f4b3d66eb0c6255c359d0c1549d17ddad76:wfo_e43223654001ee56067b5645dc54f88b24acde84565c7960248ff2243277e46c:occ_47d606c7ab372a7da3160cce` (observed revision `f642e76f509b80757d5799166a3a006ce794f868`).
- 10: `rf_wfr_f6f238da8961b4d47cf3dccf828b2f4b3d66eb0c6255c359d0c1549d17ddad76:wfo_e43223654001ee56067b5645dc54f88b24acde84565c7960248ff2243277e46c:occ_d344c63d7eb44909ec26ef03` (observed revision `f642e76f509b80757d5799166a3a006ce794f868`).
- 11: `rf_wfr_05c610fbec630a8340ea27a120af091a702a284e97bde92e906440dc3ec243ea:wfo_02bcd96ef406bcbbbd6953ac25faffe7ebb1224a1fa4680143a29cd7b004a9b3:occ_0863a97b4048d6fba3f7852d` (observed revision `fd7995de3e6dff3142509d14c775ce8b94b9c19e`).
- 12: `rf_wfr_05c610fbec630a8340ea27a120af091a702a284e97bde92e906440dc3ec243ea:wfo_02bcd96ef406bcbbbd6953ac25faffe7ebb1224a1fa4680143a29cd7b004a9b3:occ_bd7e8a3e1322ce5391cd7f86` (observed revision `fd7995de3e6dff3142509d14c775ce8b94b9c19e`).
- 13: `rf_wfr_f6f238da8961b4d47cf3dccf828b2f4b3d66eb0c6255c359d0c1549d17ddad76:wfo_e43223654001ee56067b5645dc54f88b24acde84565c7960248ff2243277e46c:occ_9911cdc4fdd5cc4f09f6cdb2` (observed revision `f642e76f509b80757d5799166a3a006ce794f868`).

## Proxy trust deployment

Set `TRUSTED_PROXY_CIDRS` to the actual immediate proxy's IPv4 /32 and IPv6 /128
addresses, plus any explicitly trusted intermediate proxy networks. Cloudflare
Tunnel deployments should identify their tunnel container's private addresses;
do not trust every container on the shared network. The default is empty. The
resolver ignores visitor-supplied left prefixes after locating the first
untrusted address from the right, including malformed left prefixes. It never
rewrites the TCP peer or uses `Forwarded`/`CF-Connecting-IP` as implicit trust.
Production network inspection is read-only; environment changes belong to the
verified release deployment. No production mutation occurred during this audit.

The forwarding-chain interpretation follows [Cloudflare's request-header
documentation](https://developers.cloudflare.com/fundamentals/reference/http-headers/#x-forwarded-for).
Cloudflare-specific header names alone never establish a trusted transport peer.
