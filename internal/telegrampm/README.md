# Qualified Telegram private messaging

This module routes ordinary bot messages to a configured forum and lets configured
panel admins reply from its per-user topic. It reuses the provider-operation
dispatcher, durable outbox and bounded Telegram executor. Inbound bodies and all files remain in Telegram. New outbound text/captions use
a bounded encrypted pending envelope for attribution; other copies use references.

- `service.go` checks first panel-entry evidence, settings and user moderation,
  then queues incoming/outgoing references. Commands and payment service messages
  remain on their existing paths. Configured admin webhook identities may refresh
  their existing account metadata; this never initializes panel-entry evidence.
- `callbacks.go` authorizes panel-admin profile refresh/moderation and recipient
  read acknowledgments against the exact conversation and message references.
- `copy.go` owns localized PM guidance and topic names; `profile_card.go` renders
  account facts, refresh/moderation controls, and the Main Mini App deep link.
  The injected profile reader builds aggregates outside Telegram's queued request callback.
- `worker.go` sequences durable topic/profile/relay/repair phases and revalidates
  configuration and identity inside each queued provider callback.
- `worker_interfaces.go` defines injected repositories and queued sender factories.
- `worker_topic.go` claims creation atomically and persists the returned topic ID
  before publishing a profile card. A restarted ambiguous creation needs review.
- `worker_profile.go` edits known cards idempotently from current account and usage
  projections; append-only card ambiguity persists separately from delivery receipts.
- `worker_relay.go` preserves formatting/captions via message references, applies
  mute state at execution time, and attributes outgoing text/captions and preserves reference-only incoming copies.
  Interrupted copies are never replayed automatically.
- `worker_repair.go` probes explicitly supplied topic/card references; returned
  Telegram chat/thread identity must match before binding. Only unsent waiters resume.
- `worker_errors.go` distinguishes preflight failures, definitive rejections and
  ambiguity. Only definitive missing/deleted-topic responses permit recreation.
  Telegram429 follows the existing bounded outbox retry policy. No second queue exists.

Topic/card references and normalized user flags are stored by the database module.
The verified-entry table is owned by accounts. Moderation is independent of paid
entitlements; PM review must not block combo controls. Admin repair is available
after inspecting Telegram because the Bot API provides no arbitrary message/topic
lookup for recovering an ambiguous create response. Keyword filters, auto replies,
backup destinations and edit synchronization are outside this module.

Selected topic locking/profile/moderation behavior was adapted from the innovation
3.0.5 version of TGbot-D1 using Go and the panel's identity/queue boundaries:
https://github.com/moistrr/TGbot-D1/blob/main/TGbot-D1%E5%88%9B%E6%96%B0%E7%89%88%E6%9C%AC3.0.5.js

Wire contracts: https://core.telegram.org/bots/api

Read state is reference-only: an inbound PM reply can infer that a delivered copy was
read. There is no per-message acknowledgment button. The admin user profile shows the
source and timestamp. This does not store message content or claim a Telegram-native
read receipt. Admin `/refund`, `/addtxb`, and `/deducttxb` commands
are accepted only inside that member's PM topic and use existing refund, ledger,
audit, and provider-operation services.

The topic profile's browser button uses `PUBLIC_BASE_URL` plus
`/admin/users/{id}`. It carries only the target reference, with no bearer or
session credentials. The browser console restores its independent admin session
or displays Mini App access-link instructions. Previously sent `admin_user_*` Mini App links
continue through the client's access-panel bridge; new cards use browser links.

Hosted regressions: `service_test.go` covers entry, supported media, commands,
forum replies and callback identity/card scope. `worker_test_helpers_test.go`
supplies reference-only repositories/senders. `worker_delivery_test.go` covers
execution-time moderation, success replay and definitive topic recovery;
`worker_recovery_test.go` covers ambiguous phases, safe rate-limit retries and
wrong-thread repair rejection. `profile_card_test.go` covers onboarding hiding,
optional coupon details, traffic, rollover, and the browser deep link. No local
automated suites are required or run.

## Administrator attribution and pending content

`content.go` captures only required outgoing text/caption, original UTF-16 entity
positions, caption placement and the authenticated sender footer. `content_test.go`
checks text/media formatting, username/name/ID fallback, Unicode and overflow.
Text uses queued sendMessage with original entities; caption-capable media uses
copyMessage caption overrides. Non-caption media and length overflow retain the
entire original and use a linked footer reply. Inbound copies remain reference-only.

`worker_content.go` restores an operation-bound encrypted envelope and checks its
mode/limits inside the queued callback. `worker_footer.go` owns a separate durable
footer phase after the original relay ID is saved. `worker_attribution_test.go`
covers attributed delivery, muted sends, footer retry without recopy, settled
replay suppression, expiry and ambiguous responses. All settings, actor, member,
source-chat and moderation guards remain execution-time checks. Append-only
uncertainty is never retried automatically.

The database's telegram_pm_payloads holds AES-GCM ciphertext bound to operation
identity; text/caption never enters outbox payloads, provider results or public PM
receipts. Settled ciphertext is erased transactionally. Unsent repair waiters
retain it for at most 24 hours. Expiry clears ciphertext while preserving a tiny
marker until operation pruning: a new job with erased content fails explicitly,
including after an explicit retry, rather than reverting to an unattributed
legacy copy. Migration 068 adds footer phases to existing unsent reference-only
outbound operations. No Telegram files or historical message bodies are mirrored.
The real PM delivery dialog displayed the expired/invalid-content guidance in English at 390px and Chinese at 1280px through Chrome DevTools MCP, with no overflow or console warnings. Delivery/recovery correctness is validated by hosted regressions; this browser evidence does not constitute a real Telegram delivery.
