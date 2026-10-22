import { computed, onMounted, reactive, readonly, shallowRef, watch } from 'vue'
import { useRouter } from 'vue-router'

import { api, ApiError } from '@/api/client'
import { restoreRef } from '@/api/cache/restore'
import { featuresApi, type PublishedOnboarding } from '@/api/features'
import type { OnboardingStep } from '@/api/types'
import { useI18n } from '@/i18n'
import { useSessionStore } from '@/stores/session'
import { notifyHaptic } from '@/utils/telegram'

type RuntimeOnboardingStep = OnboardingStep | 'membership'

function normalizeStep(step?: RuntimeOnboardingStep): OnboardingStep {
  if (!step || step === 'complete') return 'intro'
  return step === 'membership' || step === 'username' ? 'agreement' : step
}

type ErrorRecovery = (caught: unknown) => Promise<boolean>
export function useOnboarding() {
  const router = useRouter()
  const sessionStore = useSessionStore()
  const { locale, t } = useI18n()
  const step = shallowRef<OnboardingStep>(normalizeStep(sessionStore.user?.onboardingState))
  const loading = shallowRef(false)
  const error = shallowRef<string | null>(null)
  const content = shallowRef<PublishedOnboarding | null>(null)
  const form = reactive({ agreementIds: [] as string[] })
    const progress = computed(() => {
    const order: OnboardingStep[] = ['intro', 'agreement', 'complete']
    return Math.max(0, order.indexOf(step.value)) / (order.length - 1)
  })
  const allAgreementsAccepted = computed(() => Boolean(content.value?.agreements.length)
    && content.value!.agreements.every((agreement) => form.agreementIds.includes(agreement.id)))
  async function run<T>(task: () => Promise<T>, recover?: ErrorRecovery): Promise<T | undefined> {
    loading.value = true
    error.value = null
    try {
      return await task()
    } catch (caught) {
      let recovered = false
      if (recover) {
        try {
          recovered = await recover(caught)
        } catch {
          recovered = false
        }
      }
      if (recovered) return undefined
      if (caught instanceof ApiError && caught.code === 'USERNAME_UNAVAILABLE') {
        error.value = t('onboarding.usernameUnavailable')
      } else {
        const key = caught instanceof ApiError ? `errors.api.${caught.code}` : ''
        const translated = key ? t(key) : ''
        error.value = translated && translated !== key ? translated : t('onboarding.somethingWrong')
      }
      notifyHaptic('error')
      return undefined
    } finally {
      loading.value = false
    }
  }
  async function loadContent(): Promise<void> {
    restoreRef('/api/v1/onboarding/content', content, { query: { locale: locale.value } })
    const response = await run(() => featuresApi.getPublishedOnboarding(locale.value))
    if (!response) return
    content.value = response
    form.agreementIds = form.agreementIds.filter((id) => response.agreements.some((agreement) => agreement.id === id))
  }
  async function finishIntro(): Promise<void> {
    step.value = 'agreement'
    const response = await run(() => api.continueOnboarding())
    if (response) sessionStore.updateSession(response)
  }

  async function refreshAgreementState(): Promise<boolean> {
    const session = await api.getMe()
    sessionStore.updateSession(session)
    if (session.user.onboardingState === 'complete') {
      notifyHaptic('success')
      step.value = 'complete'
      await router.replace('/home')
      return true
    }
    if (session.user.onboardingState !== 'agreement') {
      step.value = normalizeStep(session.user.onboardingState)
      return false
    }
    form.agreementIds = []
    const currentContent = await featuresApi.getPublishedOnboarding(locale.value)
    content.value = currentContent
    return false
  }

  async function recoverAgreementConflict(caught: unknown): Promise<boolean> {
    if (!(caught instanceof ApiError)) return false
    if (!['AGREEMENT_OUTDATED', 'ONBOARDING_CONFLICT', 'ONBOARDING_STATE_CONFLICT'].includes(caught.code)) return false
    return refreshAgreementState()
  }

  async function acceptAgreement(): Promise<void> {
    if (!content.value || !allAgreementsAccepted.value) return
    if (sessionStore.user?.onboardingState !== 'agreement') {
      const advanced = await run(() => api.continueOnboarding())
      if (!advanced) return
      sessionStore.updateSession(advanced)
    }
    const response = await run(
      () => api.acceptAgreement(content.value!.agreementRevision, [...form.agreementIds]),
      recoverAgreementConflict,
    )
    if (!response) return
    sessionStore.updateSession(response)
    notifyHaptic('success')
    step.value = 'complete'
    await router.replace('/home')
  }

  function toggleAgreement(id: string): void {
    form.agreementIds = form.agreementIds.includes(id)
      ? form.agreementIds.filter((value) => value !== id)
      : [...form.agreementIds, id]
  }

  watch(step, () => { error.value = null })
  watch(locale, () => void loadContent())

  onMounted(() => {
    void loadContent()
  })

  return {
    step: readonly(step),
    progress,
    loading: readonly(loading),
    error: readonly(error),
    content: readonly(content),
    form,
    allAgreementsAccepted,
    finishIntro,
    acceptAgreement,
    toggleAgreement,
		loadContent,
  }
}
