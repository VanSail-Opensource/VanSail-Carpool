# Telegram formatting

- `markdown.go` escapes dynamic MarkdownV2 values, applies Telegram's
  4096-rune bound at complete escapes, bold spans, and mention links, and
  derives readable plain text for definitive Telegram parser rejections.
- `markdown_test.go` covers reserved characters and markup-safe truncation,
  including whitespace-only bold prefixes that must not become empty entities.
