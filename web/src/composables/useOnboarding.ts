import { computed, onMounted, onScopeDispose, reactive, readonly, shallowRef, watch } from 'vue'
import { useRouter } from 'vue-router'

import { api, ApiError } from '@/api/client'
import { restoreRef } from '@/api/cache/restore'
import { featuresApi, type PublishedOnboarding } from '@/api/features'
import type { OnboardingStep, Session } from '@/api/types'
import { useI18n } from '@/i18n'
import { useSessionStore } from '@/stores/session'
import { notifyHaptic } from '@/utils/telegram'

type RuntimeOnboardingStep = OnboardingStep | 'membership' | 'username'

function normalizeStep(step?: RuntimeOnboardingStep): OnboardingStep {
  if (!step || step === 'complete') return 'intro'
  return step === 'membership' || step === 'username' ? 'agreement' : step
}

type CurrentRequest = () => boolean
type ErrorRecovery = (caught: unknown, current: CurrentRequest) => Promise<boolean>
export function useOnboarding() {
  const router = useRouter()
  const sessionStore = useSessionStore()
  const { locale, t } = useI18n()
  const step = shallowRef<OnboardingStep>(normalizeStep(sessionStore.user?.onboardingState))
  const contentLoading = shallowRef(false)
  const actionLoading = shallowRef(false)
  const loading = computed(() => contentLoading.value || actionLoading.value)
  const error = shallowRef<string | null>(null)
  const content = shallowRef<PublishedOnboarding | null>(null)
  const form = reactive({ agreementIds: [] as string[] })
  let disposed = false
  let mounted = false
  let ownerRevision = 0
  let contentRevision = 0
  const progress = computed(() => {
    const order: OnboardingStep[] = ['intro', 'agreement', 'complete']
    return Math.max(0, order.indexOf(step.value)) / (order.length - 1)
  })
  const allAgreementsAccepted = computed(() => Boolean(content.value?.agreements.length)
    && content.value!.agreements.every((agreement) => form.agreementIds.includes(agreement.id)))
  function requestScope(): CurrentRequest {
    const revision = ownerRevision
    const owner = sessionStore.user?.id
    return () => !disposed && Boolean(owner) && revision === ownerRevision && sessionStore.user?.id === owner
  }
  function applySession(next: Session, current: CurrentRequest): boolean {
    if (!current() || next.user.id !== sessionStore.user?.id) return false
    sessionStore.updateSession(next)
    return true
  }
  function presentError(caught: unknown): void {
    const key = caught instanceof ApiError ? `errors.api.${caught.code}` : ''
    const translated = key ? t(key) : ''
    error.value = translated && translated !== key ? translated : t('onboarding.somethingWrong')
    notifyHaptic('error')
  }
  async function runAction(task: (current: CurrentRequest) => Promise<void>, recover?: ErrorRecovery): Promise<void> {
    const current = requestScope()
    if (!current() || actionLoading.value) return
    actionLoading.value = true
    error.value = null
    try {
      await task(current)
    } catch (caught) {
      if (!current()) return
      let recovered = false
      if (recover) {
        try {
          recovered = await recover(caught, current)
        } catch {
          recovered = false
        }
      }
      if (!recovered && current()) presentError(caught)
    } finally {
      if (current()) actionLoading.value = false
    }
  }
  async function loadContent(): Promise<void> {
    const ownerCurrent = requestScope()
    if (!ownerCurrent()) return
    const revision = ++contentRevision
    const requestedLocale = locale.value
    const current = () => ownerCurrent() && revision === contentRevision && locale.value === requestedLocale
    contentLoading.value = true
    error.value = null
    restoreRef('/api/v1/onboarding/content', content, { query: { locale: requestedLocale } })
    try {
      const response = await featuresApi.getPublishedOnboarding(requestedLocale)
      if (!current()) return
      content.value = response
      form.agreementIds = form.agreementIds.filter((id) => response.agreements.some((agreement) => agreement.id === id))
    } catch (caught) {
      if (current()) presentError(caught)
    } finally {
      if (current()) contentLoading.value = false
    }
  }
  async function finishIntro(): Promise<void> {
    await runAction(async current => {
      const response = await api.continueOnboarding()
      if (applySession(response, current)) step.value = 'agreement'
    })
  }

  async function refreshAgreementState(current: CurrentRequest): Promise<boolean> {
    const session = await api.getMe()
    if (!applySession(session, current)) return true
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
    await loadContent()
    return false
  }

  async function recoverAgreementConflict(caught: unknown, current: CurrentRequest): Promise<boolean> {
    if (!(caught instanceof ApiError)) return false
    if (!['AGREEMENT_OUTDATED', 'ONBOARDING_CONFLICT', 'ONBOARDING_STATE_CONFLICT'].includes(caught.code)) return false
    return refreshAgreementState(current)
  }

  async function acceptAgreement(): Promise<void> {
    if (!content.value || !allAgreementsAccepted.value) return
    const revision = content.value.agreementRevision
    const agreementIds = [...form.agreementIds]
    await runAction(async current => {
      if (sessionStore.user?.onboardingState !== 'agreement') {
        const advanced = await api.continueOnboarding()
        if (!applySession(advanced, current)) return
      }
      const response = await api.acceptAgreement(revision, agreementIds)
      if (!applySession(response, current)) return
      notifyHaptic('success')
      step.value = 'complete'
      await router.replace('/home')
    }, recoverAgreementConflict)
  }

  function toggleAgreement(id: string): void {
    form.agreementIds = form.agreementIds.includes(id)
      ? form.agreementIds.filter((value) => value !== id)
      : [...form.agreementIds, id]
  }

  watch(step, () => { error.value = null })
  watch(() => content.value?.agreementRevision, (next, previous) => {
    if (next !== previous) form.agreementIds = []
  }, { flush: 'sync' })
  watch(locale, () => void loadContent())
  watch(() => sessionStore.user?.id, (owner) => {
    ownerRevision++
    contentRevision++
    actionLoading.value = false
    contentLoading.value = false
    content.value = null
    form.agreementIds = []
    error.value = null
    step.value = normalizeStep(sessionStore.user?.onboardingState)
    if (mounted && owner) void loadContent()
  }, { flush: 'sync' })

  onMounted(() => {
    mounted = true
    void loadContent()
  })
  onScopeDispose(() => { disposed = true })

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
