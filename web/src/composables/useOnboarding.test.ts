import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { defineComponent, h, nextTick } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError } from '@/api/http'
import type { PublishedOnboarding } from '@/api/features'
import type { OnboardingStep, Session } from '@/api/types'
import { setLocale } from '@/i18n'
import { useSessionStore } from '@/stores/session'

const api = vi.hoisted(() => ({ continueOnboarding: vi.fn(), getMe: vi.fn(), acceptAgreement: vi.fn() }))
const features = vi.hoisted(() => ({ getPublishedOnboarding: vi.fn() }))
const replace = vi.hoisted(() => vi.fn())
const notifyHaptic = vi.hoisted(() => vi.fn())
vi.mock('@/api/client', async () => ({ api, ApiError: (await import('@/api/http')).ApiError }))
vi.mock('@/api/features', () => ({ featuresApi: features }))
vi.mock('@/api/cache/restore', () => ({ restoreRef: () => false }))
vi.mock('@/api/cache/preload', () => ({ preloadSession: vi.fn(), stopSessionPreload: vi.fn() }))
vi.mock('vue-router', () => ({ useRouter: () => ({ replace }) }))
vi.mock('@/utils/telegram', () => ({ notifyHaptic }))

import { useOnboarding } from './useOnboarding'

function response(state: OnboardingStep): Session {
  return { authenticated: true, user: {
    id: 'user-1', telegramId: '42', firstName: 'Mira', lastName: '', telegramUsername: '', displayName: 'Mira', username: 'mira', role: 'user',
    onboardingState: state, groupJoined: false, channelJoined: false, policyAcceptedAt: null,
    agreementRevision: 1, recoveryReason: '', createdAt: '2026-08-31T00:00:00Z', updatedAt: '2026-08-31T00:00:00Z',
  } }
}
function published(revision = 1, locale: PublishedOnboarding['locale'] = 'en'): PublishedOnboarding {
  return { locale, welcomeRevision: 1, agreementRevision: revision, welcome: [], agreements: [{ id: 'terms', icon: 'shield-check', title: 'Terms', body: `Revision ${revision}` }] }
}
function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>(accept => { resolve = accept })
  return { promise, resolve }
}
const wrappers: VueWrapper[] = []
function mountOnboarding(state: OnboardingStep = 'intro') {
  const pinia = createPinia()
  const session = useSessionStore(pinia)
  session.session = response(state)
  const updateSession = vi.spyOn(session, 'updateSession')
  let onboarding!: ReturnType<typeof useOnboarding>
  const wrapper = mount(defineComponent({ setup() { onboarding = useOnboarding(); return () => h('div') } }), { global: { plugins: [pinia] } })
  wrappers.push(wrapper)
  return { onboarding, wrapper, session, updateSession }
}

describe('useOnboarding', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    setLocale('en')
    features.getPublishedOnboarding.mockResolvedValue(published())
  })
  afterEach(() => { for (const wrapper of wrappers.splice(0)) wrapper.unmount() })

  it('advances from intro to agreements without requesting a username', async () => {
    api.continueOnboarding.mockResolvedValue(response('agreement'))
    const { onboarding } = mountOnboarding()
    await flushPromises()
    await onboarding.finishIntro()
    expect(api.continueOnboarding).toHaveBeenCalledWith()
    expect(onboarding.step.value).toBe('agreement')
  })

  it.each(['owner cleared', 'unmounted'] as const)('ignores pending advancement after %s', async reason => {
    const pending = deferred<Session>()
    api.continueOnboarding.mockReturnValueOnce(pending.promise)
    const { onboarding, wrapper, session, updateSession } = mountOnboarding()
    await flushPromises()
    const advancing = onboarding.finishIntro()
    if (reason === 'owner cleared') session.clear()
    else wrapper.unmount()
    pending.resolve(response('agreement'))
    await advancing
    expect(updateSession).not.toHaveBeenCalled()
    expect(onboarding.step.value).toBe('intro')
    expect(replace).not.toHaveBeenCalled()
  })

  it('keeps the newest locale content when requests finish out of order', async () => {
    const initial = deferred<PublishedOnboarding>(), latest = deferred<PublishedOnboarding>()
    features.getPublishedOnboarding.mockReturnValueOnce(initial.promise).mockReturnValueOnce(latest.promise)
    const { onboarding } = mountOnboarding()
    setLocale('zh-CN')
    await nextTick()
    latest.resolve(published(2, 'zh-CN'))
    await flushPromises()
    initial.resolve(published())
    await flushPromises()
    expect(onboarding.content.value).toEqual(published(2, 'zh-CN'))
    expect(onboarding.loading.value).toBe(false)
  })

  it('requires renewed consent when the agreement revision changes', async () => {
    const { onboarding } = mountOnboarding('agreement')
    await flushPromises()
    onboarding.toggleAgreement('terms')
    features.getPublishedOnboarding.mockResolvedValueOnce(published(2))
    await onboarding.loadContent()
    expect(onboarding.form.agreementIds).toEqual([])
    await onboarding.acceptAgreement()
    expect(api.acceptAgreement).not.toHaveBeenCalled()
  })

  it('keeps actions exclusive while content refresh completes and submits the reviewed revision', async () => {
    const advance = deferred<Session>(), accepted = deferred<Session>()
    api.continueOnboarding.mockReturnValueOnce(advance.promise)
    api.acceptAgreement.mockReturnValueOnce(accepted.promise)
    const { onboarding } = mountOnboarding()
    await flushPromises()
    onboarding.toggleAgreement('terms')
    const accepting = onboarding.acceptAgreement()
    features.getPublishedOnboarding.mockResolvedValueOnce(published(2))
    await onboarding.loadContent()
    expect(onboarding.loading.value).toBe(true)
    onboarding.toggleAgreement('terms')
    await onboarding.acceptAgreement()
    expect(api.continueOnboarding).toHaveBeenCalledOnce()
    advance.resolve(response('agreement'))
    await flushPromises()
    expect(api.acceptAgreement).toHaveBeenCalledExactlyOnceWith(1, ['terms'])
    accepted.resolve(response('complete'))
    await accepting
    expect(replace).toHaveBeenCalledExactlyOnceWith('/home')
  })

  it('recovers already committed agreement acceptance without resubmitting it', async () => {
    api.acceptAgreement.mockRejectedValueOnce(new ApiError(409, { code: 'ONBOARDING_CONFLICT', message: 'Already committed' }))
    api.getMe.mockResolvedValueOnce(response('complete'))
    const { onboarding } = mountOnboarding('agreement')
    await flushPromises()
    onboarding.toggleAgreement('terms')
    await onboarding.acceptAgreement()
    expect(onboarding.step.value).toBe('complete')
    expect(onboarding.error.value).toBeNull()
    expect(api.acceptAgreement).toHaveBeenCalledOnce()
    expect(replace).toHaveBeenCalledExactlyOnceWith('/home')
  })

  it('ignores conflict recovery after its owner is cleared', async () => {
    const pending = deferred<Session>()
    api.acceptAgreement.mockRejectedValueOnce(new ApiError(409, { code: 'AGREEMENT_OUTDATED', message: 'Outdated' }))
    api.getMe.mockReturnValueOnce(pending.promise)
    const { onboarding, session, updateSession } = mountOnboarding('agreement')
    await flushPromises()
    onboarding.toggleAgreement('terms')
    const accepting = onboarding.acceptAgreement()
    await flushPromises()
    session.clear()
    pending.resolve(response('complete'))
    await accepting
    expect(session.user).toBeNull()
    expect(updateSession).not.toHaveBeenCalled()
    expect(replace).not.toHaveBeenCalled()
    expect(notifyHaptic).not.toHaveBeenCalled()
  })
})
