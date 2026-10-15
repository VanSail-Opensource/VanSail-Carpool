# Composables

- `motionPresets.ts` owns the shared timing tokens and critically damped layout spring.
- `useMotionPreferences.ts` is the central reduced-motion boundary for Motion Vue components; it returns reduced-safe transitions and bounded offsets without storing animation state in components.

- `dashboardNodeUsage.ts` owns the dashboard's date-range validation, cached usage display, latest-request guard and disposal.

Page loaders restore typed session snapshots through `api/cache/restore.ts` before
their existing requests. Cached content renders during refresh; backend responses
replace it through the same state and error handling. Query and cursor keys remain
separate. Action quotes and permission decisions still await the backend.

- `useSquadAddition.ts` owns active-term optional-squad filtering, time-adjusted quote freshness, activation-code handoff, balance recovery, and idempotent commit feedback.

- `useActivity.ts` loads games, draws, and check-in actions.
- `useActivity.test.ts` checks that ambiguous draw failures reuse the same idempotency key and confirmed draws release it.
- `useAffiliateCentre.ts` loads member metrics and fixed five-row referral pages.
- `useAdminDatabase.ts` coordinates protected database review and mutation flows.
- `useAdminSection.ts` provides latest-request-safe administrator list operations and cursor accumulation.
- `useCatalog.ts` prepares catalog selection, quote freshness, full paid add-on guards, automatic-renewal catalog blocking, and user-scoped session drafts.
- `catalogQuoteState.ts` owns the request-versioned authoritative catalog quote and its selection fingerprint.
- `catalogDraft.ts` reads, validates through callers, writes, and clears user-scoped catalog drafts in session storage.
- `catalogLoader.ts` loads catalog, balance, coupon wallet, and validated user-scoped drafts while reporting whether the latest response was applied.
- `useCatalog.test.ts` covers catalog selection behavior.
- `useCouponRedemption.ts` redeems a coupon code for the guided purchase flow.
- `useCouponRedemption.test.ts` covers guided redemption normalization and idempotency reuse.
- `useAutoRenewal.ts` owns the owner-scoped automatic-renewal status load, eligibility, and server toggle update.
- `useDisplayCurrency.ts` keeps the persisted member display choice and read-only CNY/USD rate snapshot in one reactive source of truth.
- `useAutoRenewal.test.ts` covers the server-ineligible toggle constraint.
- `useClipboard.ts` provides safe clipboard feedback.
- `useConnectionScan.ts` creates one idempotent provider scan and polls its owner-scoped progress every second without repeating an ambiguous start mutation.
- `useConnectionScan.test.ts` covers accepted scan polling and ambiguous-start idempotency reuse.
- `useConnectionBlocks.ts` loads active blocks, retains per-target block/unblock idempotency keys across ambiguous failures, and delegates receipt polling without replaying mutations.
- `useConnectionBlocks.test.ts` covers key reuse, unblock polling, and active-list refresh.
- `useCoupons.ts` manages coupon-wallet redemption and server-confirmed grant discards.
- `useCoupons.test.ts` covers confirmed and failed coupon-wallet discards.
- `useDashboard.ts` loads dashboard summaries and provides disposal-safe bounded per-node UTC traffic state to its Home descendants.
- `useRolloverDetail.ts` fetches fresh aggregate rollover details when the active Home ride opens, with cancellation-safe retry/reset state and localized errors.
- `useEmby.ts` manages Emby setup and preferences.
- `useImageZoom.ts` owns bounded focal-point pinch, pan, wheel, double-tap, button, interaction, and reset state for image canvases.
- `useImageZoom.test.ts` covers zoom bounds, reset, and two-finger gesture state.
- `useIntroSequence.ts` drives the onboarding intro sequence.
- `useIntroSequence.test.ts` covers intro timing and completion.
- `useOnboarding.ts` coordinates the intro, username, and agreement steps, including idempotent agreement recovery for returning users.
- `useCommunityMembership.ts` refreshes canonical Telegram membership and strict active-combo access, opens requested-space invites, and unregisters its Telegram activation refresh handler on teardown.
- `useCommunityAccess.ts` shares the read-only server-confirmed valid-combo gate between the shell and Home Around TX entrances.
- `useCommunityMembership.test.ts` covers invite opening, error feedback, Telegram activation refresh, and event cleanup; `useOnboarding.test.ts` covers direct intro-to-username-to-agreement progression.
- `usePaymentOrder.ts` coordinates durable checkout/cancellation receipts with payment-sheet presentation states.
- `usePaymentCountdown.ts` owns a disposal-safe one-second payment-window countdown used only for presentation and replacement prompts.
- `usePaymentConfiguration.ts` owns exact TXB bounds, amount conversion, and selected payment method state.
- `usePaymentOrderOperations.ts` retains stable mutation keys and polls payment provider-operation receipts without replaying provider calls.
- `usePaymentTarget.ts` opens accepted provider targets and read-only polls authoritative payment status.
- `paymentOrderHelpers.ts` validates exact payment bounds and replacement candidates, classifies terminal orders, and renders address-first provider QR payloads.
- `usePaymentOrder.test.ts` covers payment-order state transitions.
- `usePaymentReturn.ts` polls a provider-returned payment order until it is confirmed or terminal.
- `usePaymentReturn.test.ts` covers durable payment-return confirmation and pending polling.
- `useOperationReceipt.ts` performs bounded, read-only polling for durable member operation receipts and stops on every terminal or review state.
- `useDurableCommand.ts` retains stable UUID keys for ambiguous command retries, invalidates disposed submissions, and delegates accepted receipt polling to `useOperationReceipt.ts`.
- `usePurchaseOperations.ts` owns authoritative first-term refund quotes, conflict refresh, mutation idempotency and receipt state. Automatic resets remain an account setting; no member manual-reset command is exposed.
- `usePurchaseOperations.test.ts` covers quote conflict refresh, idempotency reuse, server-owned refund eligibility, and immediate automation persistence.
- `useQuestionnaireImport.ts` previews and settles questionnaire imports with reset-safe mutation and polling generations.
- `useQuestionnaireImport.test.ts` covers import analysis and settlement.
- `useQuestionnaires.ts` manages member questionnaire participation.
- `useNodeGeocheck.ts` loads an image-only cached node Geocheck for a minimal UUID/name target shared by Statistics and Catalog, only after explicit selection, and clears its state on close.
- `useNodeCompensation.ts` coordinates revisioned policy, filtered cursor history, and stable-key reviewed outage commands.
- `useNodeCompensation.test.ts` covers idempotency-key reuse after an ambiguous review failure.
- `useNodeGeocheck.test.ts` covers selected-target loading, localized unavailability, and close reset behavior.
- `useStatistics.ts` independently loads the 30-minute aggregate snapshot and
  ten-second node cache while preserving last-good data across partial failures.
- `useTelegramBackButton.ts` coordinates one native Telegram BackButton across route and overlay owners, adds soft navigation feedback, and removes every handler on teardown.
- `useTelegramProtection.ts` reference-counts active destructive, busy, or dirty workflows and enables native closing confirmation plus vertical-swipe suppression only while needed.
`useNodeGeocheck.ts` preserves squad context, skips disabled requests, and checks live squad settings before displaying images from enabled contexts.

- `useActivity.ts` reads eligibility live, tracks refresh state and hides reward controls when verification refresh fails; a rejected check-in refreshes boost state while retaining its localized refusal.

`useTelegramSettingsButton.ts` owns the native Settings button with SDK discovery and lifecycle cleanup. Reset automation now belongs to the Settings page instead of `usePurchaseOperations.ts`.
- `useIPLookupAvailability.ts` shares account-scoped feature availability across shell and Around TX entrances.
