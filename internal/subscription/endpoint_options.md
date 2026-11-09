# Administrator client endpoint options

`Proxy.EndpointTuning` carries `EndpointOptions` from the published endpoint's
encrypted `overrides.clientOptions`. It is separate from owner-specific member
`Proxy.Tuning`. Neither option set, credentials nor rendered documents are
ordinary proxy JSON fields or new database mirrors.

## Interfaces and ownership

The existing `tuning_catalog.json` is the single field/type/path inventory.
Ownership is `member`, `administrator` or `shared`. Existing member fields remain
shared; new server-coordinated client fields are administrator-owned. The
generator emits separate Go/API option types; member `ConnectionOverrides`,
`ActiveTuning` and retired-key removal retain their original allowlist.

`EndpointFields(proxy)` returns current-inbound controls with ownership, bounds,
dependencies and inherited XHTTP mode. `ValidateEndpointOptions(options, proxy)`
checks explicit values, current context, cross-field constraints and the pinned
Xray stream builder. It reports field codes without silently pruning saved
administrator intent. `EndpointCompatibility(options, proxy)` independently
reports six client formats, essential exclusions and optional omitted keys.

TLS/Reality, WebSocket, gRPC, HTTPUpgrade, Hysteria hopping and XHTTP client
extras use scalar, enum, header-row and inclusive range controls. Separate XHTTP
downloads use flat typed controls projected into a structured download stream.
There is no arbitrary JSON property, private-key input or listener-only setting.
TLS `allowInsecure:true` is removed in the pinned Xray parser; compatibility
inputs permit false/inherit, and certificate pin/name verification has its own
controls. Header rows are bounded and case-insensitive duplicates are rejected.
Older inherited `allowInsecure:true` is projected as secure false on both legs,
with a fixed-off diagnostic. Its encrypted source document is not rewritten;
older endpoints remain repairable and selectable under the pinned policy.

## Export order and compatibility

Each renderer clones the source stream and merges typed administrator choices
with inherited extras, nested XMUX and download objects. Xray's explicit `extra`
replaces flat extras as a whole: ignored flat values are never resurrected.
Identity/path/mode stay at their native level; session fields use `sessionID*`.
An explicit positive XMUX max-connections choice replaces only an inherited
max-concurrency choice, and conversely. Child legs follow the same rule while
retaining unrelated XMUX defaults. Explicit zeros remain explicit; two explicit
positive choices fail save validation and are omitted with diagnostics when
reading an older saved document. The member allowlist and permissions stay intact.
Existing client mappers run before final typed choices. The unchanged member
allowlist overlays compatible member preferences last; MPTCP/external XHTTP Mux
policy remains in force. Input maps and credentials are never mutated.

Xray, native/base64, sing-box, Mihomo and Stash each use their documented keys.
Essential connection/identity fields that cannot be represented exclude only
that endpoint from that format. Optional unsupported controls are omitted with
diagnostics. Saved fields that became inapplicable after a protocol change stay
saved, are omitted at export and report context; new saves reject that mismatch.
Inherited download transport is an essential capability only while that leg is
active; explicit disable, another active transport or stream-one can safely omit
an old leg without excluding the current endpoint.
Base protocol/transport capability is checked with the existing pure renderer on
the canonical administrator copy, with both option pointers cleared. Preview
metadata and actual endpoint selection therefore share adapter behavior without
recursion or I/O.
An unsupported peer does not prevent rendering the remaining endpoints.

Hysteria port sets remain compact. Xray uses `finalmask.udp` with `udphop`,
`intervalRemote`, `remotePorts` and seconds `interval`; other masks are retained.
Official Hysteria URIs place comma/hyphen ports in the authority, including IPv6,
and have no custom timing query. sing-box uses colon ranges in `server_ports`,
removes `server_port`, and supports duration `hop_interval`/`hop_interval_max`.
Mihomo uses `ports` and fixed/ranged `hop-interval`; Stash supports only a fixed
integer interval. Unsupported optional random timing is omitted, never collapsed
into a different fixed policy. Empty explicit ports disable inherited hopping.
Opaque inherited hopping modes remain Xray-only and cannot be silently edited.
Unrepresented peer-decoded masks exclude that format; one-way UDP noise and TCP
fragmentation remain optional omissions with diagnostics. VLESS native links
preserve the official `fm` document without adding an arbitrary JSON control.

Stash is independent of Mihomo: unsupported XMUX/padding-placement extensions
are not emitted. HTTPUpgrade maps to Mihomo's documented WebSocket upgrade flag;
Stash excludes it. sing-box excludes XHTTP. Native VLESS links carry the complete
documented XHTTP `extra`; VMess JSON links use their own transport/TLS schema.

## Implementation and hosted checks

`endpoint_values.go` owns strict bounded decoding; `endpoint_fields.go` owns
current context; `endpoint_validation.go` owns semantic validation;
`endpoint_projection.go` owns immutable native merging;
`endpoint_compatibility.go` owns client diagnostics; `endpoint_render.go` owns
client export order. `endpoint_hysteria_values.go` and `endpoint_hysteria.go`
own compact hopping parsing and format adapters.
`endpoint_dependencies.go` shares current context/mode rules across all stages;
`endpoint_omissions.go` keeps optional output consistent with diagnostics;
`endpoint_finalmask.go` separates optional one-way masks from required decoding.
`endpoint_download.go` materializes destination/path/session/security defaults
required by the pinned Xray dialer, including partial inherited download legs.

`endpoint_hysteria_test.go`, `endpoint_export_review_test.go` and
`endpoint_validation_review_test.go` cover all formats, boundary ports, timing,
inheritance, opaque masks, native IPv6 authorities, member/admin isolation,
session identity, protocol changes, unsupported peers and mutation isolation.
No external API or second queue is introduced. Runtime client version support
still matters; non-Xray documents are structurally projected, while previews
also use the actual pinned Xray parser.

Primary references, reviewed 2026-11-09:

- [Pinned Xray client transport parser](https://github.com/XTLS/Xray-core/blob/a4a3dff5b20c/infra/conf/transport_method.go)
  and [TLS/Reality parser](https://github.com/XTLS/Xray-core/blob/a4a3dff5b20c/infra/conf/transport_security.go).
- [XHTTP design and extra](https://github.com/XTLS/Xray-core/discussions/4113),
  [VLESS URI proposal](https://github.com/XTLS/Xray-core/discussions/716) and
  [VMess share-link schema](https://github.com/2dust/v2rayN/wiki/Description-of-VMess-share-link).
- [Hysteria URI](https://v2.hysteria.network/docs/developers/URI-Scheme/) and
  [port hopping](https://v2.hysteria.network/docs/advanced/Port-Hopping/).
- [sing-box transports](https://sing-box.sagernet.org/configuration/shared/v2ray-transport/)
  and [Hysteria2](https://sing-box.sagernet.org/configuration/outbound/hysteria2/).
- [Mihomo transports](https://wiki.metacubex.one/en/config/proxies/transport/),
  [Hysteria2](https://wiki.metacubex.one/en/config/proxies/hysteria2/) and
  [Stash protocol contracts](https://stash.wiki/en/proxy-protocols/proxy-types).
