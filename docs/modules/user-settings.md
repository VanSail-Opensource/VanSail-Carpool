# User Settings

Settings is available on `/settings`, desktop/mobile navigation and the native
Telegram Settings button. The page inherits the premium dark theme and English
and Simplified Chinese catalogs; language, currency and reset automation reuse
their existing state and endpoints.

`GET/PATCH /api/v1/me/preferences` exposes ten preference flags and sparse PAYG squad choices. Missing database
rows read as defaults; partial saves merge atomically. All notification groups,
referral attribution and combined node prices default on. Around TX and Activity
default off and require a currently active combo. Purchase transitions clear
entrance preferences so a later purchase cannot resurrect an earlier choice.
The frontend scopes cached state to the account and ignores stale responses.

The shared notification worker and affiliate worker read the latest category
before private delivery. Disabled jobs finish without sending; lookup errors
retry. Command replies, group broadcasts and financial/event audit facts remain
independent. The referral welcome omits inviter attribution when opted out.
Personal security notices also follow the account category; their operator
copies and payment-provider health alerts retain operational delivery behavior.

`GET/PUT /api/v1/me/group-member-tag` reads and updates Telegram directly through
the existing upstream queue. Membership and bot `can_manage_tags` rights are
checked at execution. A missing member hides the editor; unsupported roles or
missing bot permissions disable it. Empty clears the tag, Unicode length is
limited to 16, and Telegram remains authoritative for emoji restrictions.

## Browser evidence, 2026-10-07

Validation used the actual Chrome DevTools MCP 0.26.0 server over a local stdio
bridge, loading production components with constructed account/API fixtures.
Mobile was emulated at 390 × 844 with touch; desktop at 1280 × 900.

- Initial/default, loading, inactive-combo, unjoined-group, missing bot permission,
  failed preference save, translated language and currency selection were checked.
- Partial PATCH payloads, notification toggling, automatic reset PUT, native
  Settings navigation, tag save and empty-tag clearing were observed.
- Around TX and Activity appeared after opt-in and disappeared after opt-out.
- The same core combo displayed 125.00 TXB with a selected paid squad and
  100.00 TXB with node inclusion disabled; included/free squads were not charged.
- Both viewports had no horizontal overflow. Switch labels toggle their controls
  and the switch hit area was confirmed beyond the visual thumb. Final console
  checks had no warnings/errors. API behavior used mocks, not live upstreams.

Browser inspection also exposed the shell's old Boolean `storage` prop. It now
uses Nuxt UI's documented `storage="local"` and `persistent` prop, keeping
Telegram persistence disabled without a runtime warning.

Regression cases are authored for hosted CI. No local automated suite was run.

PAYG settings add a master switch and independent per-squad opt-outs, all default on. Existing prepaid credit retains its original expiry. Saving queues native reconciliation atomically and does not change paid Combo entitlement. Disabled admission cannot trigger connection purchases; delivered traffic remains accountable.

## Browser evidence, 2026-10-25

Chrome DevTools MCP validation used production Settings/catalog components with constructed account/API fixtures at 320px, 390px and 1440px. PAYG master-off preserved squad opt-outs, disabled subordinate controls and restored saved choices. Failed saves retained confirmed state; empty/error/loading paths were exercised. Member-only DIY and admin-only console entrances retained their routes and 60px touch rows. PAYG controls remain exclusively in Settings; the PAYG page has no settings link.

Combo stayed hidden/inert at 50ms, 500ms and 760ms, and became visible after its introduction. Slow loading, rapid switching, scroll/resize, reduced motion and single semantic haptic delivery were inspected. No production operations or local automated suites were used.
