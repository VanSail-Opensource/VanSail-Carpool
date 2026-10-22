import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const api = vi.hoisted(() => ({ continueOnboarding: vi.fn(), getMe: vi.fn(), acceptAgreement: vi.fn() }))
const features = vi.hoisted(() => ({ getPublishedOnboarding: vi.fn() }))
const session = vi.hoisted(() => ({ user: { onboardingState: 'intro' }, updateSession: vi.fn() }))

vi.mock('@/api/client', () => ({ api }))
vi.mock('@/api/features', () => ({ featuresApi: features }))
vi.mock('@/stores/session', () => ({ useSessionStore: () => session }))
vi.mock('vue-router', () => ({ useRouter: () => ({ replace: vi.fn() }) }))
vi.mock('@/utils/telegram', () => ({ notifyHaptic: vi.fn() }))

import { useOnboarding } from './useOnboarding'

function response(state: 'agreement' | 'complete') {
  return {
    authenticated: true,
    user: {
      id: 'user-1', telegramId: '42', firstName: 'Mira', lastName: '', telegramUsername: '', username: 'mira', role: 'user',
      onboardingState: state, groupJoined: false, channelJoined: false, policyAcceptedAt: null,
      agreementRevision: 1, recoveryReason: '', createdAt: '2026-08-31T00:00:00Z', updatedAt: '2026-08-31T00:00:00Z',
    },
  }
}

describe('useOnboarding', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    session.user = { onboardingState: 'intro' }
    features.getPublishedOnboarding.mockResolvedValue({ welcome: [], agreements: [], revision: 1 })
  })

  it('advances from intro to agreements without requesting a username', async () => {
    api.continueOnboarding.mockResolvedValue(response('agreement'))
    let onboarding!: ReturnType<typeof useOnboarding>
    const wrapper = mount(defineComponent({ setup() { onboarding = useOnboarding(); return () => h('div') } }))
    await flushPromises()

    await onboarding.finishIntro()
    expect(api.continueOnboarding).toHaveBeenCalledWith()
    expect(onboarding.step.value).toBe('agreement')
    wrapper.unmount()
  })
})
