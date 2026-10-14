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
