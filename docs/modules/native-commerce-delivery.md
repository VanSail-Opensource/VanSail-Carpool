# Native commerce and subscription delivery

The panel owns native identities, desired configuration, access and accounting.
X-Node is the sole control plane. Reconciled imported usage remains immutable accounting evidence.
Emby and the existing payment integrations retain their separate boundaries.

## Commerce audit

| Flow | Accounting and settlement boundary |
| --- | --- |
| Resets | Current combined counters support reset reconciliation during migration. New reset periods retain the purchase reference; unfinished predecessor journals remain pending. Updating an entitlement changes its cadence without replacing its usage. |
| Renewal | Existing atomic rollover-credit/debit settlement creates one paid successor. Deferred predecessor settlement selects its own purchase periods and cannot revoke successor streams. Native calculation provenance survives the transaction. |
| Rollover | Native purchase-owned usage is counted once, without apportioning those owned daily bytes a second time at intraday boundaries. Allowance and strict remaining thresholds retain whole-term semantics; the new calculation version is `native-term-v1`. Source exports retain their documented daily resolution. |
| Refunds | Member zero-use checks include all same-purchase reset periods and earlier imported source history. Unfinished or overflowing accounting cannot qualify. Legacy-only checks also read term history. Administrator suggestions retain the net-paid cap and manual override. |
| Compensation | Native readiness uses fresh ready reports; legacy administrative disable state remains authoritative until handover. Approved extensions change expiry and queued continuity without resetting the existing combined quota. |

Refunds and rollover use purchase-owned native settlement barriers and retain imported evidence.
Reset prechecks use the current counter and do not borrow that financial fence.
Previously settled financial records are not repriced. Saturating arithmetic and
exact decimal serialization prevent overflow or browser rounding from fabricating
unused traffic. Live telemetry never enters the database.

Hosted regression sources include `xnode/commerce_test.go`,
`app/xnode_purchase_operations_test.go`, `rollover/native_term_test.go`,
`database/native_rollover_version_test.go` and existing reset, automatic renewal,
refund and compensation suites. No local automated test suite was executed.

## Retired controls

Detector ingestion, operational secrets, jobs and installer were retired by 078.
Migration 079 removes the squad image-check flag and disables executable account
relinking and scheduled host-remark operations. Their handlers, APIs, UI, locale
keys, dedicated tests and obsolete example payload are removed. Historical audit,
incident and financial facts remain, along with proven detector-ban release work.
Operator-owned old reporter files follow the explicit removal instructions in
`detector-retirement.md`; the panel does not remotely delete host files.

## Member and administrator interfaces

The member subscription center provides owner-scoped routing/outbound JSON
editors, accessible endpoint templates, revision-checked saves and advertised
subscription formats. Homepage live rates, counts and limitation reasons use
memory-only snapshots. Unavailable coverage and errors remain visible.

The independent browser console provides readable collector values and units,
typed node commands, durable speedtest outcomes, certificate-to-inbound bindings,
feature switches and editor diagnostics. Invalid JSON cannot submit a stale parsed
value. Save failures retain the editor and draft. Configuration previews include
selected snippets and use the pinned Xray validator.

## Subscription contracts

Administrator templates support Xray, sing-box and Clash Meta. Structural markers
insert only the requesting member's eligible proxies; expressions are never
evaluated. JSON syntax, names, references, selector cycles and output limits are
checked. Xray previews additionally use the pinned native core. sing-box and Clash
checks are structural rather than execution in those client runtimes.

Templates persist encrypted documents and subscription references in the existing
settings store. Defaults, format switches and per-subscription assignments use an
optimistic revision. Previews use synthetic credentials, and generated member
documents are never persisted. Explicit imported per-host Xray overrides retain
priority; endpoint editing can remove an override when switching to shared policy.

The source bridge expands upstream computed snippets and captures only referenced
Xray client templates. Conversion preserves client settings, and preflight repairs
older imported endpoints without overwriting administrator overrides. Xray output
contains independently selectable endpoint configurations. New SS-2022 presets use
AES-256; imported methods and credentials remain unchanged. TLS verification choices
and WebSocket host fields survive format conversion. Live rates use bytes/second;
peer speedtests explicitly report Mbps. Large integer counters cross the browser
boundary as decimal strings.

`url` identifies the public subscription page; `downloadUrl` identifies its raw
configuration API. The copied public URL also negotiates client content from an
explicit format or recognized client agent. Browser navigation retains HTML.
Private headers and `Vary` protect the content distinction, and disabled/inactive
formats have no usable download or import action.

Client-field references: [sing-box TLS](https://sing-box.sagernet.org/configuration/shared/tls/),
[Clash Meta TLS](https://wiki.metacubex.one/en/config/proxies/tls/).

## Validation and release evidence

Chrome DevTools MCP exercised constructed member, public and browser-admin data
at 390px and 1280px CSS widths. Evidence covers populated/empty/loading/error
states, retry, routing insertion and saves, invalid JSON, format links, detailed
metrics, peer command results, certificate selection, template validation/preview,
assignment and retained drafts after a revision conflict. Mock throughput is not
a measurement of a production peer. Client runtime compatibility beyond the
structural checks remains the responsibility of the selected client.

Static gates include generated contracts, structure/localization checks, frontend
type checking and lint, Go vet, module verification and diff review. Hosted CI and
signed-release results are recorded in the final delivery response.
