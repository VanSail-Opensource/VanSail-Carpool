# Schema shards

`schemas-02.yaml` includes nullable member-specific renewal admission metadata
on Catalog. Its deadline is a UI projection; checkout revalidates the term.
Squad read/write models also carry optional `paygState`, defaulting new overrides
to normal; omitted values on older-client writes preserve administrator intent.

`xnode-providers.yaml` defines the supported flag and public zone ID/name pairs
for request-local administrator discovery. DDNS saves names, not Cloudflare IDs.

`xnode.yaml` defines panel-owned DDNS endpoint bindings. Built-in address families
accept multiple endpoint IDs; the deprecated single-ID fields remain readable
during rolling client upgrades.

`browser-auth.yaml` defines public entrance configuration,
one-use handoff inputs/results, and the discriminated browser
administrator/member/anonymous session state.

`schemas-24.yaml` defines owned squad controls, recoverable combo state and the
full-duration early-activation quote.

Schema definitions are grouped into bounded sequential shards. The root component registry references each definition by JSON Pointer.

- `schemas-01.yaml`: `ID`, `Timestamp`, `DecimalInteger`, `ApiError`, `TelegramAuthRequest`, `OnboardingStep`, `User`, `AuthState`, `JoinInvite`, `CommunityMembership`, `UsernameRequest`, `AgreementRequest`, `WelcomeMessage`, `OnboardingAgreement`, `LocalizedOnboardingContent`, `PublishedOnboarding`, `OnboardingBundle`
- `schemas-02.yaml`: `OnboardingDraftWrite`, `OnboardingPublishRequest`, `Money`, `ResetStrategy`, typed squad profiles, `SquadProduct` with live accessible nodes, `Combo`, nullable-provider `CatalogNode`, `Catalog`, `ComboWrite`, `SquadProductWrite`
- `schemas-03.yaml`: `ProxyNode`, `PurchaseRequest`, `PurchaseQuote`, `EntitlementStatus`, `Entitlement`, `Purchase`, `SubscriptionRotation`, `TopNode`, `Statistics`, `StatisticPoint`
- `schemas-04.yaml`: `StatisticSlice`, `AdminStatistics`, `Dashboard`, `LedgerKind`, `LedgerEntry`, `PageInfo`, `LedgerPage`, `PaymentMethodID`, `PaymentOrderMethodID`, `PaymentProvider`, `PaymentStatus`, `PaymentOrderRequest`
- `schemas-05.yaml`: `PaymentOrder`, activity games, instant-only member `LuckyDraw` previews, typed draw reward fields, and resolved `ActivityResult`
- `schemas-06.yaml`: `GroupMessageRewardStatus`, `ActivityOverview`, activity requests/settings, mode-specific `LuckyDrawPrize` and `LuckyDrawWrite`, `LuckyDrawAdmin`, `LuckyDrawForecast`, and coupon kinds
- `schemas-07.yaml`: `Coupon`, `CouponWrite`, `CouponList`, `CouponGrant`, `CouponGrantList`, `CouponRedeemRequest`, `CouponRedemption`, `QuestionnaireStatus`, `Questionnaire`, `QuestionnaireWrite`, `QuestionnaireList`
- `schemas-08.yaml`: `QuestionnaireParticipation`, `ActiveQuestionnaire`, `QuestionnaireHistoryItem`, `QuestionnaireHistoryList`, `QuestionnaireImportStatus`, `QuestionnaireImportAnalysis`, `QuestionnaireImportPreview`, `QuestionnaireImportAnalyzeRequest`, `QuestionnaireSettlementReport`, `QuestionnaireImportState`, `EmbyRating`, `EmbyLibrary`, `EmbyAccount`
- `schemas-09.yaml`: `EmbyOverview`, `EmbyPreferencesRequest`, `EmbySetupRequest`, `EmbyPasswordRequest`, `EmbyAccountList`, `AdminSetting`, `AdminSettingCreate`, `AdminSettingUpdate`, `AdminUserSummary`, `AdminUserPage`, `BalanceAdjustmentRequest`, `ReasonRequest`, `EntitlementPage`, `AdminEntitlement`
- `schemas-10.yaml`: `PaymentReturnStatus`, `PaymentPage`, `AdminPaymentOrder`, `RefundRequest`, `Refund`, `RefundPage`, `DatabaseColumn`, `DatabaseTable`, `DatabaseTableList`, `DatabaseBlobValue`, `DatabaseValue`, `DatabaseKey`, `DatabaseValues`, `DatabaseRow`, `DatabaseRowPage`, `DatabaseFilterOperator`
- `schemas-11.yaml`: `DatabaseFilter`, `DatabaseQueryRequest`, `DatabaseMutationAction`, `DatabaseMutationReviewRequest`, `DatabaseMutationApplyRequest`, `DatabaseCompatibilityUpdate`, `DatabaseMutationReview`, `DatabaseMutationResult`, `RestoreRequest`, `RestoreOperation`, `BackupRun`, `JobStatus`, `Job`, `JobPage`
- `schemas-12.yaml`: `AuditEvent`, `AuditPage`, `TelegramUpdate`, `FlexibleDecimal`, `BEPusdtNotification`, `BEPusdtUnsignedNotification`, `HealthStatus`, `RuntimeReadiness`
- `schemas-13.yaml`: `CourtesyCredit`
- `schemas-14.yaml`: `DashboardNodeUsage`
- `schemas-15.yaml`: `AutoRenewal`, `AutoRenewalUpdate`, `AutoRenewalFailure`, stable-ID `PaymentProfile`, and `PaymentProfileWrite`
- `schemas-16.yaml`: live `RolloverWindow` and `RolloverProjection` aggregate response schemas
- `schemas-17.yaml`: durable operation and payment-operation receipts, signed connection projections, active IP blocks, account-wide reset automation, refund quotes, and billing amount limits
- `schemas-18.yaml`: admin entitlement/combo mutations, exact refund amounts, coupon and provider-account commands, minute-normalized bulk extension requests with a legacy-days compatibility input, and aggregate user detail including active IP blocks and temporary-ban state
- `schemas-20.yaml`: affiliate member projections, tier rewards, bot identity, and optimistic administrator writes
- `schemas-21.yaml`: `PaymentMethod` with provider display name and pending-order-aware `BalanceResponse`
- `schemas-22.yaml`: revisioned node-compensation configuration, outage projections, review writes, recipient-safe event pages

- `schemas-06.yaml` adds the live group boost status and effective activity reward amounts; counts and links are nullable when verification is unavailable.

`schemas-23.yaml` defines notification preferences, typed partial updates and live group-member tags.
`schemas-23.yaml` also defines the administrator refund suggestion and nullable usage details.

`schemas-25.yaml` defines first-entry CAPTCHA state and the bounded single-use verification token request.

`schemas-26.yaml` defines private-message conversations, moderation/repair requests, paged inventory and bounded delivery references. Telegram numeric IDs remain decimal strings.

`connectivity.yaml` defines the seven credential-free connectivity configuration, account, attempt, batch, host-result, snapshot and cursor-history contracts. Nullable fields remain present; X-Node account references are canonical numeric IDs.

`uptime.yaml` describes neutral-gap timelines, the credential-free home summary and member-only subscription host/squad projections, without diagnostic timing or HTTP status fields.
- `ip-lookup-config.yaml` defines independent provider/geolocation ordering, allowance, signed pricing and response-only free cache previews. Submission uses only the signed quote fields.
- `ip-lookup-report.yaml` defines compact merged facts, source provenance, refusal evidence, nullable MaxMind fields and owner-scoped receipts with the requested IP and cache match.
`ip-lookup-live.yaml` defines source timestamps/status, bounded BGP paths/topology, ASN metrics, nullable contextual scores, block counts, PTR records, and Cloudflare traffic windows.
- `ip-lookup-progress.yaml` describes derived check progress and typed live stream events.

`ip-lookup-enhanced.yaml` defines named inventories, compact matches, per-address subnet coverage, protocol evidence, durable probe progress and the identity-free shared progress response. Coordinates remain in the report facts for map rendering.

`ip-lookup-enhanced.yaml` defines independent administrator source cache status and combined unique counts. `ip-lookup-report.yaml` adds optional AI suitability ratings and datacenter warnings while retaining verdict compatibility.

- `usdt.yaml` ? Typed built-in USDT administration, quota, scan and review contracts.

`xnode.yaml` defines authoritative nodes, per-node policy, global configuration, shared snippets, live metrics and enrollment. Live metrics remain nonpersistent.

`api-tokens.yaml` defines token metadata, explicit permissions, expiry input and one-time issuance responses.

`schemas-19.yaml` defines product statistics snapshots, distributions and live node metrics.

- `xnode-services.yaml` defines native standalone identities and explicit access terms.

`xnode-firewall.yaml` defines shared ban evidence, quote-bound paid exemption requests and durable grant responses.


`xnode-routing.yaml` describes typed managed choices, matcher presets, scoped graph
paths, destination previews and durable deployment phases without private secrets.

`xnode-profile.yaml` defines native per-node availability, decimal-string counters and exact seven-day accounting with separately identified non-directional legacy bytes.

`xnode-speedtests.yaml` defines saved-route start input, durable pair outcomes and
measurements/estimates. Traffic counters and probe credentials are never exposed.

`xnode-diy.yaml` defines owner DIY views, previews, both-revision writes and save receipts. `xnode-diy-overrides.yaml` is generated from the typed client-only tuning catalog.

`payg.yaml` defines owner-only PAYG summaries, reporting timestamps, availability states and decimal-string traffic/wallet values. Squad-product contracts carry optional billing fields for compatibility with existing clients.

Member preferences expose default-on `paygEnabled` and a sparse `paygSquads` map; PATCH merges supplied squad flags. PAYG availability includes `disabled` without altering financial totals.
