# Common components

- `ConfirmDialog.vue` provides the shared Nuxt UI confirmation modal with soft cancellation, rigid confirmation, heavy destructive confirmation feedback, and an opt-in truly centered header without close-control spacing.
- `CountryFlag.vue` renders bundled Iconify flags for common nodes, including the Netherlands, and an ISO regional-indicator flag fallback for every other valid country code.
- `InlineNotice.vue`, `OperationStatusNotice.vue`, `SkeletonBlock.vue`, and `StatusBadge.vue` provide status feedback, including durable operation polling failures. Notice, receipt/error state swaps, and badge presence are component-owned Motion with opacity-first reduced-motion fallbacks; global styles do not animate semantic mounts.
- Operation notices use simultaneous presence because a receipt, polling failure
  and retry action can coexist. Sequential single-child presence hides recovery
  controls with newer Motion versions.
- `LanguageSwitcher.vue` controls the active locale.
- `DualCurrencyMoney.vue` renders an authoritative TXB amount with an optional second-line approximate CNY/USD amount for checkout totals.
- `SwitchField.vue`, `TxbAmountField.vue`, and `MarkdownEditorField.vue` provide domain form fields; `TxbAmountField.vue` keeps exact TXB entry and can add a bounded `USlider` for member funding without weakening minor-unit validation.
- `SwitchField.vue` uses a native label for its entire 52px row, so tapping the copy or surrounding space toggles the associated Nuxt UI switch once. The name and optional help remain separate accessible associations, and disabled switches retain their native guard.
- `MarkdownContent.vue` safely renders allowlisted Markdown.
- `MarkdownEditorField.vue` wraps Nuxt UI `UEditor` and `UEditorToolbar`, retaining Markdown persistence and the sanitized member preview. `markdownToolbar.ts` owns localized formatting actions; `MarkdownStyleControls.vue` applies safe text styling and `markdownStyle.ts` round-trips the existing color/size directives through the editor.
- `MarkdownContent.test.ts` covers Markdown sanitization and rendering.

`JsonDocumentField.vue` is the shared localized JSON field. External value changes reset invalid drafts; parsed edits emit update/validity events. It owns no node or administrator state.

`MetricLineChart.vue` renders bounded rate, count, byte-total, money and percentage
series with localized captions. Callers can supply a forecast label appropriate
to their metric; the default remains the usage forecast label.

- `LivePrice.vue`: Compact localized indicator beside an applicable dynamic amount.

- `PricingQuoteNotice.vue`: Localized calculating, delayed, stale and unavailable quote states, manual retry and precise validity.
