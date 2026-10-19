# Subscription rendering

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

Xray rendering and connectivity probes share the existing ordered mapper in
`internal/connectivity`. Sing-box and Mihomo apply their own mapper operations.
Unsupported protocol or transport combinations return errors instead of producing
broken imports. Host exclusions are applied before rendering.

- `types.go`: Defines private client projections, stable tags and Xray rendering.
- `native.go`: Renders credential-bearing native proxy links.
- `singbox.go`: Projects accounts and transports into sing-box JSON.
- `clash.go`: Projects accounts and transports into Clash Meta YAML.
- `render_test.go`: Covers VLESS credentials, REALITY and composite SS-2022 keys.

`template.go` expands structural proxy/name markers into administrator client documents without evaluating expressions. Xray returns one selectable profile per endpoint; explicit per-host templates take precedence. `template_validation.go` checks references and converts source Xray templates; the fleet service adds pinned-core Xray validation. sing-box and Clash validation is structural, with client runtime compatibility depending on their supported fields. `template_test.go` covers insertion, isolation of endpoint profiles, imported settings, TLS choices and WebSocket host translation. Output profiles and member credentials are never persisted.

Public interfaces: `Template`, `DefaultTemplate`, `PreviewTemplate`, `RenderTemplate` and `ConvertXrayTemplate`. Extend format-specific `Proxy` renderers alongside reference validation. YAML conversion uses the existing ghodss/yaml dependency; Xray previews additionally use the panel pinned native validator. These functions perform no outbound requests.

`template_markers.go` bounds structural insertion depth and checks the single proxy marker. `template_references.go` rejects dangling rules and cyclic selectors. Generated output is bounded to 4,096 endpoints and 8 MiB; the renderer never drops unsupported endpoints silently. Explicit source exclusions are retained.

Clash and the distinct Stash template share structural YAML validation. Generated REALITY hosts always include `reality-opts.support-x25519mlkem768: true`, following https://stash.wiki/en/proxy-protocols/proxy-types. Stash uses its own mapper and SNI field. Member credentials are rendered on demand; previews use synthetic credentials.
