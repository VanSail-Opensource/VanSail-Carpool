# Subscription rendering

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
