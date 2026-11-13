# Subscription rendering

`endpoint_options.md` documents administrator client controls, catalog ownership,
strict save validation and independent client-format adaptation.
`endpoint_types.gen.go` defines the separate generated `EndpointOptions` type.
`endpoint_values.go` provides bounded typed decoding; `endpoint_fields.go` exposes
current inbound fields; `endpoint_validation.go` applies bounds/dependencies and
pinned native validation. `endpoint_projection.go` merges immutable native
streams; `endpoint_render.go` applies administrator then member export choices;
`endpoint_compatibility.go` reports essential exclusions and optional omissions.
`endpoint_omissions.go` removes unsupported inherited/mapped optional values so
client output agrees with diagnostics.
`endpoint_dependencies.go` shares effective-mode/context dependency decisions
between save validation, runtime projection and client diagnostics.
`endpoint_finalmask.go` excludes unrepresented essential masks per client;
documented VLESS native finalmask remains intact.
`endpoint_download.go` materializes safe destination/session/security defaults
and deep-merges explicit structured download legs without recursive downloads.
`endpoint_xmux.go` switches only an opposing inherited XMUX choice while keeping
two explicit positive choices invalid, shared by member/admin and download merge.
`endpoint_xmux_test.go` and `endpoint_xmux_inheritance_test.go` verify both mode
directions, inherited defaults, explicit zero, conflicts, peer retention and
member overlay using the pinned native stream builder without network traffic.
`endpoint_hysteria_values.go` parses compact port/timing policy;
`endpoint_hysteria.go` maps independent client formats.
`endpoint_hysteria_test.go`, `endpoint_export_review_test.go` and
`endpoint_validation_review_test.go` cover hosted export/validation boundaries.
`endpoint_adapter_test.go` checks full Hysteria/VMess/native TLS documents, Reality
alias precedence and optional/required finalmask compatibility.
`endpoint_download_test.go` verifies partial download inheritance against the
pinned memory stream/destination types and unsupported download isolation.
`endpoint_download_compatibility_test.go` checks disabled, stale-transport and
stream-one download applicability before essential client exclusion, plus
metadata agreement with real protocol/transport/client-mapper adapters.
`endpoint_tls_policy_test.go` preserves legacy source ciphertext semantics while
projecting the pinned secure false policy on both client legs.

`compatibility.go` is the shared endpoint-selection boundary for metadata,
synthetic previews and live rendering. Explicit endpoint exclusions and typed
unsupported client protocols/transports are reported separately from invalid
templates. A failed host projection/mapping omits only that endpoint. Compatible
endpoints continue rendering; template failures remain visible. Format references: [sing-box VLESS](https://sing-box.sagernet.org/configuration/outbound/vless/)
and [Stash protocols](https://stash.wiki/en/proxy-protocols/proxy-types).

Clash means current Mihomo-compatible clients, including FlClash/Clash Meta.
`xhttp.go` exports VLESS XHTTP with TLS/Reality, padding/upload options, XMUX and
separate downloads using the [Mihomo transport contract](https://wiki.metacubex.one/en/config/proxies/transport/).
Stash's documented XHTTP representation is supported too. Unsupported hosts are
omitted per client format rather than rejecting a mixed subscription.
`xhttp_test.go` covers Mihomo/Stash XHTTP fields, TLS/Reality combinations and
per-host omission when a projection fails.

`Proxy` is an owner-scoped, process-local client projection. Renderers produce
native links, Xray JSON, sing-box JSON and Clash Meta YAML. Credential maps,
mapper sources and transport settings are excluded from ordinary JSON encoding.
The HTTP layer owns bearer authorization, revocation and no-store headers.
Split XHTTP pairs must share the same billed principal, node, listener, protocol,
path and session options, and use distinct published hosts. PAYG inventory uses
its owner-specific host IDs in loading, validation, saved references and export;
no Combo or whole-GB wallet threshold gates those client metadata projections.

Xray rendering uses the subscription-owned ordered mapper. Sing-box and Mihomo apply their own mapper operations.
Unsupported protocol or transport combinations return errors instead of producing
broken imports. Host exclusions are applied before rendering.

- `types.go`: Defines private client projections, stable tags and Xray rendering.
- `native.go`: Renders credential-bearing native proxy links.
- `singbox.go`: Projects accounts and transports into sing-box JSON.
- `clash.go`: Projects accounts and transports into Clash Meta YAML.
- `render_test.go`: Covers VLESS credentials, REALITY and composite SS-2022 keys.

`native_encryption_test.go` verifies that administrator-owned VLESS encryption
material survives native links, member tuning and endpoint projection without
alteration. Native links follow the
[Xray share-link encryption contract](https://github.com/XTLS/Xray-core/discussions/716).
Mihomo and Stash retain their supported encryption field; the published
[sing-box VLESS schema](https://sing-box.sagernet.org/configuration/outbound/vless/)
has no encryption representation, so those endpoints are explicitly excluded
while compatible peers continue rendering.

`template.go` expands structural proxy/name markers into administrator client documents without evaluating expressions. Xray returns one selectable profile per endpoint; explicit per-host templates take precedence. `template_validation.go` checks references and converts source Xray templates; the fleet service adds pinned-core Xray validation. sing-box and Clash validation is structural, with client runtime compatibility depending on their supported fields. `template_test.go` covers insertion, isolation of endpoint profiles, imported settings, TLS choices and WebSocket host translation. Output profiles and member credentials are never persisted.

Public interfaces: `Template`, `DefaultTemplate`, `PreviewTemplate`, `RenderTemplate` and `ConvertXrayTemplate`. Extend format-specific `Proxy` renderers alongside reference validation. YAML conversion uses the existing ghodss/yaml dependency; Xray previews additionally use the panel pinned native validator. These functions perform no outbound requests.

`template_markers.go` bounds structural insertion depth and checks the single proxy marker. `template_references.go` rejects dangling rules and cyclic selectors. Generated output is bounded to 4,096 endpoints and 8 MiB; the renderer never drops unsupported endpoints silently. Explicit source exclusions are retained.

Clash and the distinct Stash template share structural YAML validation. Generated REALITY hosts always include `reality-opts.support-x25519mlkem768: true`, following https://stash.wiki/en/proxy-protocols/proxy-types. Stash uses its own mapper and SNI field. Member credentials are rendered on demand; previews use synthetic credentials.

`empty_http.go` validates response-only templates: final status 200-599, omitted
status 403, no document/body. They never enter client-format defaults or exports.

Client connection tuning uses the pinned Xray source and official Xray,
sing-box, Mihomo and Stash contracts independently. `tuning_catalog.json` is the
field/path allowlist; `tuning_types.gen.go` and `tuning_catalog.gen.go` are
regenerated by the web script. `tuning_values.go` retains absent/false/zero and
sentinels with strict typed JSON. `tuning_validation.go` checks bounds, headers,
structured headers and range conflicts. `tuning_fields.go` obtains pinned
fingerprint names and platform/version metadata. `tuning_compatibility.go`
shares applicability across previews, metadata, host links and exports;
`tuning_apply.go` applies explicit compatible values after administrator mapping.
`tuning_xhttp.go` preserves inherited XHTTP fields and protected download identity.
`tuning_test.go` covers inheritance, all formats, protected identity, native-link
limits, validation and mixed endpoint support.

Tuning attaches only at member export and host-link boundaries. Routing, transit,
monitoring and isolated probes retain the original configuration. Unsupported
options are omitted with reasons while compatible hosts stay included. Native
links expose only documented fp/spx parameters for VLESS/Trojan. Stash does not
inherit Mihomo XMUX mappings. SagerNet/Mihomo multiplex and UoT cannot be enabled
against native Xray endpoints that do not advertise those server protocols.
Addresses, SNI, keys/certificates, ECH identity, ciphers, server transport placement,
DNS, TUN and listeners remain administrator-owned. Listener-only socket fields,
proxy chaining and key logging are outside the member allowlist. Schema checks
for non-Xray clients cannot replace execution in each client's version/platform;
Xray previews additionally use the actual pinned parser.

Sources: https://xtls.github.io/en/config/outbound.html ;
https://github.com/XTLS/Xray-core/discussions/4113 ;
https://sing-box.sagernet.org/configuration/shared/dial/ ;
https://sing-box.sagernet.org/configuration/shared/multiplex/ ;
https://wiki.metacubex.one/en/config/proxies/ ;
https://stash.wiki/en/proxy-protocols/proxy-types .

`tuning_rows.go` rejects missing/null structured row fields. `tuning_native.go` limits share-link parameters to documented client fields.

`split.go` derives a combined XHTTP stream from two current hosts reaching the
same native inbound and session path/placement. The upload mode is independent
of ordinary-host tuning. Selected download host security/address/port remain
administrator-owned; the selected download leg inherits its administrator transport settings. Native/base64
links carry the documented XHTTP `extra`; Xray and Mihomo/Stash render their
supported download settings. Unsupported formats omit only the combined entry.
The ordinary entries remain in all inventories.

`tuning_dependencies.go` shares the catalog's mode/parent requirements with Go
validation, exports and frontend metadata. Inactive dependent values are ignored
before validation and excluded from settings persistence; the browser alone keeps
the hidden draft. Context availability comes from current owner-scoped hosts.
Upstream session/mode reference: https://github.com/XTLS/Xray-core/discussions/4113.

`split_test.go` covers session identity, profile modes, format exports and inactive
field exclusion in hosted CI.

Inherited endpoint modes are included in dependency metadata. The frontend and
owner validation use these alongside explicit draft values, hiding packet-upload
controls when all applicable inherited hosts use stream-one. With
no authoritative host inventory, generic future-host preferences are retained.

`client_policy.go` applies export policy after mappers: MPTCP is disabled on both XHTTP legs and external Mux/SMUX is omitted for XHTTP, while built-in XMUX remains. `client_policy_test.go` covers mapper precedence and supported Mihomo fingerprints. `tuning_fingerprints.go` follows https://github.com/MetaCubeX/mihomo/blob/Meta/component/tls/utls.go (reviewed 2026-10-23). `tuning_retirement.go` strips only known retired options from older saved documents. `split_name.go` merges shared name terms and canonical V4V6 families; `split_name_test.go` covers shared and unrelated names.

`tuning_restrictions.go` confines version/platform guidance to the client format that implements it. Inapplicable inherited fields do not add empty compatibility rows.

Internal `Proxy` attribution retains its published endpoint and billing principal for PAYG routing selection. These private references and credentials never enter ordinary JSON. Distinct squad credentials on one endpoint receive stable export IDs. A nonpositive wallet balance removes PAYG entries from fresh owner exports without changing internal inventory or paid Combo entries.

`Proxy.BillingMode` explicitly distinguishes hourly secondary principals from
PAYG; a nonzero principal ID alone never determines wallet/export policy. Hourly
hosts use enrollment-scoped stable IDs and preserve already prepaid access at
zero wallet balance. Their speed allowance is not a hard traffic quota; member
subscription metadata uses unlimited total and the exact latest paid expiry.

`template_names.go` allocates sing-box tags and Mihomo/Stash names after client
mapping and before expanding `SubscriptionNames`. PAYG principals sharing a
published endpoint retain their separate credentials; duplicate mapped labels
receive compact numeric suffixes, such as `Tokyo (2)`, assigned in public-ID
order without exposing hashes. Template-owned names and built-in
policies are reserved, and existing endpoint labels are preserved when unique.
Only the ephemeral rendered document changes; inventory, routing, accounting
and saved templates remain authoritative. Regression coverage lives in
`template_names_test.go` and `internal/xnode/payg_subscription_test.go`.

`member.go` defines owner-scoped host and subscription projections. `client_mapper.go` and `client_mapper_path.go` implement bounded ordered client mapping without retaining credentials; `client_mapper_test.go` covers copying, mutation isolation and rejected paths.
