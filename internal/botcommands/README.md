# Telegram bot commands

This package owns Telegram slash-command parsing, localized reply copy, and
MarkdownV2-safe formatting. HTTP webhook code supplies authenticated domain data;
the package never calls persistence or providers directly.

- `command.go` parses every slash command before group analytics can record it.
- `locale.go` selects English or Simplified Chinese and owns command copy.
- `format.go` owns shared status-card and deduction formatting.
- `pm_admin.go` owns localized PM-topic admin command descriptions and refund/balance replies.
- `format_account.go` renders titleless start/balance replies, check-in reward
  comparisons, and safe member mentions.
- `format_subscription.go` renders emoji-led subscription/combo summaries and
- `traffic.go` owns 1024-based byte labels, the eight-cell usage bar, and the
  five-node 30-cell traffic distribution.
- `markdown.go` delegates dynamic escaping and safe truncation to the shared
  Telegram formatter while preserving the command package compatibility surface.
- `command_test.go` covers suffix parsing, unknown-command exclusion, locale
  selection, PM admin command parsing, and bounded output.
- `format_test.go` covers both locales, exact start/balance copy, check-in and
- `traffic_test.go` covers byte parsing/formatting, clamping, bar allocation,
  displayed-node totals, empty distributions, and the 4096-character limit.

- Group reward commands use localized, MarkdownV2-safe boost-required and verification-unavailable copy from `locale.go`; `format.go` renders the refusal without replying to ordinary group messages.
- `group_boost_test.go` verifies both refusal reasons have localized, escaped English and Chinese copy.

The localized /start reply explains direct messaging after panel entry verification; command menus and payment paths remain unchanged.

Display-currency compatibility regression: `display_currency_test.go`.
