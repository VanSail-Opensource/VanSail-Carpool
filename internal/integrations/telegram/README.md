# Telegram integration

This package contains the Telegram Bot API and Mini App authentication surface used by VanSail Carpool. Bot tokens remain confined to request URLs inside the transport, while returned and transport errors are sanitized before propagation.

- `client.go` defines the sanitized API error, client options, injected transport, and validated client construction.
- `transport.go` performs bounded Bot API calls, decodes envelopes, sanitizes failures, and validates URLs and secrets.
- `webhooks.go` configures webhooks and the Mini App menu button and verifies webhook secrets in constant time.
- `bot.go` registers command menus and sends, edits, or deletes MarkdownV2 messages; publication returns the Bot API message ID for durable raffle progress updates. Definitive entity-parse rejections get one plain-text retry while other Telegram errors remain visible.
- `memberships.go` manages join-request invites, approvals, explicit declines, revocation, and canonical chat membership lookup.
- `ChatMember.Present` normalizes Telegram membership states so the HTTP
  adapter can welcome only configured-group absent-to-present transitions.
- `stars.go` creates Stars invoice links, answers pre-checkout queries, lists transactions, and requests refunds.
- `types.go` defines Telegram user, chat, update, membership, invite, payment, and Stars wire contracts.
- `initdata.go` verifies signed Telegram Mini App init data with freshness and replay-resistant field handling.
- `doc.go` supplies the package documentation.
- `client_test.go` covers Bot API wire methods, sanitized errors, update decoding, and webhook verification.
- `bot_test.go` verifies MarkdownV2 send-message payloads and successful Message result decoding.
- `redirect_test.go` verifies provider redirects cannot receive the bot token or request body.
- `initdata_test.go` covers init-data signatures, time bounds, token validation, and optional Telegram signature fields.
- `README.md` documents the package layout and security boundary.

- `boosts.go` implements the documented administrator-only `getUserChatBoosts` method and typed response subset; `types.go` includes authenticated group-message `sender_boost_count` metadata.
- `boosts_test.go` checks request identity, response decoding, validation and sender boost metadata.

`member_tag.go` implements documented `setChatMemberTag`, including empty-tag removal and Unicode length validation.

`boosts.go` also models documented chat_boost/source identities. Webhooks subscribe to boost additions and changes; anonymous giveaway sources remain optional.

`messages.go` holds minimal command/payment/thread/media-presence contracts without persistence. `forum.go` implements documented getChat/createForumTopic/copyMessage and optional acknowledgment markup on copied messages. `topic_profiles.go` implements thread-addressed profile creation, idempotent edits with returned thread identity, callback/URL buttons and callback answers. New methods execute only through the app queue adapters.
`forum_test.go` covers documented forum, copy, profile and callback wire methods, retained-caption behavior and returned thread identity in hosted CI.

`pm_content.go` defines Telegram message entities, UTF-16 sizing and attributed
PM text/reply requests. SendPMText preserves entities without parse-mode
reinterpretation or truncation and validates the returned destination identity.
`pm_content_test.go` covers the queued adapter's wire contract in hosted CI.
forum.go adds optional caption overrides only for new attributed outbound media.
