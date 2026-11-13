# Onboarding components

- `OnboardingFlow.vue` coordinates intro and agreement steps. Username collection is retired.
- `IntroSequence.vue` and `AgreementPanel.vue` render each step; agreement cards render server-authored color and an optional localized page title. Intro advancement exposes pending/error state and an explicit retry after the timed sequence finishes, without replaying the sequence or submitting duplicate actions.
- `IntroSequence.test.ts` covers retrying failed advancement and pending-state exclusion.
- `agreementIcons.ts` maps server-owned icon keys to external Iconify names.
- `useOnboardingMainButton.ts` mirrors the decisive step action to Telegram's native MainButton with an in-app fallback.
- `useOnboardingMainButton.test.ts` verifies native-button synchronization and teardown.

Onboarding distinguishes soft navigation/opening, medium native actions, rigid completion, and semantic agreement selection.
