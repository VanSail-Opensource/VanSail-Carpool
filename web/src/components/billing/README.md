# Billing components

Built-in stablecoin instructions place the localized gas-fee notice beside the
exact amount: network fees are additional and must not reduce the received amount.

- `BalancePaymentSheet.vue` resumes valid pending orders before provider selection and drives coupon funding plus the durable provider-operation lifecycle from Home.
- `BalancePaymentConfiguration.vue` is the billing flow entrypoint. It keeps provider selection, channel selection, coupon redemption, and order actions coordinated without changing the payment API contract.
- `PaymentProviderStep.vue` renders the first Add TXB step, choosing one stable provider-account profile or the coupon funding source.
- `PaymentChannelStep.vue` renders the second Add TXB step with a range-aware Nuxt UI slider and exact decimal amount field, then starts or rechecks the provider operation without overlapping controls.
- `PaymentCryptoChannelPicker.vue` splits BEPUSDT selection into white-logo currency and network controls backed by discovered rails.
- `CryptoPaymentInstructions.vue` renders the compact exact crypto amount, address QR, copy action, countdown, explicit replacement, and user cancellation without exposing a provider URL.
- `paymentOptions.ts` owns the typed provider and channel option contracts and resolves the shared monochrome logos in `web/public/assets/payments/`. Currency and network artwork uses white foregrounds with transparent backgrounds and cutouts so every consumer has the same appearance without CSS filters. Provider glyphs use the same white foreground in `billing-02.css`; selection checks retain their semantic feedback colors. Add new channel or network logos to the existing resolver and keep their artwork monochrome.
- `PaymentReceiptDetails.vue` renders the browser-safe provider-return receipt with localized amount, payment ID, provider, channel, status, and timing metadata using Nuxt UI surfaces; exact amounts and identifiers wrap instead of truncating.
- `BalancePaymentSheet.test.ts` verifies provider labels stay distinct from provider-owned payment channels, such as EZPay and Alipay, and verifies the two-step transition.

Provider and channel changes use semantic selection feedback with a 140ms Motion check reveal and an opacity-only reduced-motion path; navigation is soft, order confirmation is rigid, retries are light, and order cancellation is heavy.
