# Public subscription page

`main.ts` boots a third, public experience for `/subscription/:token`. It reuses
the browser theme and Nuxt UI while starting no administrator or Mini App session.
The bootstrap branch is selected before checking Telegram launch context.

`PublicSubscriptionPage.vue` displays owner-scoped usage and expiry, available
formats, copy, QR and client import actions. `usePublicSubscription.ts` loads
metadata without cookies/signatures, aborts superseded requests and derives the
raw subscription URL from the current origin. QR generation stays process-local.
Client schemes follow sing-box's graphical client contract and Clash Verge Rev.
Invalid/revoked bearer links and inactive access have localized states.

Public responses, downloads and metadata use no-store and no-referrer headers.
Tokens and credentials must never enter shared caches, logs, analytics or local
storage. The existing member and administrator roots remain independent.

Public and authenticated member controls use the canonical `downloadUrl` configuration API. The `url` field remains the browser landing page. Older metadata falls back to the existing token API; inactive or disabled formats have no import/download action.

Base64 is the primary format. Stash, sing-box, Clash, Xray and explicit plain native links retain separate selections; enabled metadata controls availability.

`PublicSubscriptionPage.vue` retains explicit-format selection and shows server-provided tuning omissions for the selected client.
