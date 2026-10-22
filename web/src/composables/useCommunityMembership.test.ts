import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { CommunityMembership } from '@/api/types'

const mocks = vi.hoisted(() => ({
  checkCommunityMembership: vi.fn(),
  createCommunityInvite: vi.fn(),
  getTelegramWebApp: vi.fn(),
  notifyHaptic: vi.fn(),
  openExternalLink: vi.fn(),
  updateSession: vi.fn(),
  session: null as unknown,
}))

vi.mock('@/api/client', () => ({ api: mocks }))
vi.mock('@/stores/session', () => ({ useSessionStore: () => ({ session: mocks.session, updateSession: mocks.updateSession }) }))
vi.mock('@/utils/telegram', () => ({
  getTelegramWebApp: mocks.getTelegramWebApp,
  notifyHaptic: mocks.notifyHaptic,
  openExternalLink: mocks.openExternalLink,
}))

import { useCommunityMembership } from './useCommunityMembership'

function member(activeCombo = true): CommunityMembership {
  return {
    activeCombo, groupJoined: false, channelJoined: false,
    user: {
      id: 'user-1', telegramId: '42', firstName: 'Mira', lastName: '', telegramUsername: '', displayName: 'Mira', username: 'mira', role: 'user',
      onboardingState: 'complete', groupJoined: false, channelJoined: false, policyAcceptedAt: '2026-08-31T00:00:00Z',
      agreementRevision: 1, recoveryReason: '', createdAt: '2026-08-31T00:00:00Z', updatedAt: '2026-08-31T00:00:00Z',
    },
  }
}

function mountCommunity() {
  let community!: ReturnType<typeof useCommunityMembership>
  const wrapper = mount(defineComponent({ setup() { community = useCommunityMembership(); return () => h('div') } }))
  return { community, wrapper }
}

describe('useCommunityMembership', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.session = null
  })
  afterEach(() => vi.clearAllMocks())

  it('refreshes after Telegram activation and unregisters the listener', async () => {
    let activated: ((...args: unknown[]) => void) | undefined
    const telegram = { onEvent: vi.fn((_event: string, callback: (...args: unknown[]) => void) => { activated = callback }), offEvent: vi.fn() }
    mocks.getTelegramWebApp.mockReturnValue(telegram)
    mocks.checkCommunityMembership.mockResolvedValue(member())
    const { wrapper } = mountCommunity()
    await flushPromises()

    activated?.()
    await flushPromises()

    expect(mocks.checkCommunityMembership).toHaveBeenCalledTimes(2)
    wrapper.unmount()
    expect(telegram.offEvent).toHaveBeenCalledWith('activated', expect.any(Function))
  })

  it('opens only the requested invite link and keeps failures visible', async () => {
    mocks.getTelegramWebApp.mockReturnValue(undefined)
    mocks.checkCommunityMembership.mockResolvedValue(member())
    mocks.createCommunityInvite.mockResolvedValue({ url: 'https://t.me/+channel', expiresAt: '2026-08-31T12:30:00Z' })
    const { community, wrapper } = mountCommunity()
    await flushPromises()

    await community.join('channel')
    expect(mocks.createCommunityInvite).toHaveBeenCalledWith('channel')
    expect(mocks.openExternalLink).toHaveBeenCalledWith('https://t.me/+channel')

    mocks.createCommunityInvite.mockRejectedValueOnce(new Error('offline'))
    await community.join('group')
    expect(community.error.value).toBeTruthy()
    expect(mocks.notifyHaptic).toHaveBeenCalledWith('error')
    wrapper.unmount()
  })
})
