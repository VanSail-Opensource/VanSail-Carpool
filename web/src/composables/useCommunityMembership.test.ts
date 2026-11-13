import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { defineComponent, h } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { CommunityMembership, JoinInvite } from '@/api/types'
import { useSessionStore } from '@/stores/session'

const mocks = vi.hoisted(() => ({
  checkCommunityMembership: vi.fn(),
  createCommunityInvite: vi.fn(),
  getTelegramWebApp: vi.fn(),
  notifyHaptic: vi.fn(),
  openExternalLink: vi.fn(),
}))

vi.mock('@/api/client', async () => ({ api: mocks, ApiError: (await import('@/api/http')).ApiError }))
vi.mock('@/api/cache/restore', () => ({ restoreRef: () => false }))
vi.mock('@/api/cache/preload', () => ({ preloadSession: vi.fn(), stopSessionPreload: vi.fn() }))
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
  const pinia = createPinia()
  const session = useSessionStore(pinia)
  session.session = { authenticated: true, user: member().user }
  const updateSession = vi.spyOn(session, 'updateSession')
  let community!: ReturnType<typeof useCommunityMembership>
  const wrapper = mount(defineComponent({ setup() { community = useCommunityMembership(); return () => h('div') } }), { global: { plugins: [pinia] } })
  return { community, wrapper, session, updateSession }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>(accept => { resolve = accept })
  return { promise, resolve }
}

describe('useCommunityMembership', () => {
  beforeEach(() => {
    vi.resetAllMocks()
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

  it('keeps the newest membership response when activation overlaps the initial read', async () => {
    const initial = deferred<CommunityMembership>()
    const refresh = deferred<CommunityMembership>()
    mocks.checkCommunityMembership.mockReturnValueOnce(initial.promise).mockReturnValueOnce(refresh.promise)
    const { community, wrapper, updateSession } = mountCommunity()
    const loading = community.load()
    const current = { ...member(), channelJoined: true }
    refresh.resolve(current)
    await loading
    initial.resolve(member(false))
    await flushPromises()

    expect(community.membership.value).toEqual(current)
    expect(community.loading.value).toBe(false)
    expect(community.refreshing.value).toBe(false)
    expect(updateSession).toHaveBeenCalledOnce()
    wrapper.unmount()
  })

  it('cannot restore a cleared owner from a delayed membership response', async () => {
    const pending = deferred<CommunityMembership>()
    mocks.checkCommunityMembership.mockReturnValueOnce(pending.promise)
    const { community, wrapper, session, updateSession } = mountCommunity()
    session.clear()
    pending.resolve(member())
    await flushPromises()

    expect(session.user).toBeNull()
    expect(community.membership.value).toBeNull()
    expect(updateSession).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it.each(['owner cleared', 'unmounted'] as const)('does not open a retired invite after %s', async reason => {
    const pending = deferred<JoinInvite>()
    mocks.checkCommunityMembership.mockResolvedValue(member())
    mocks.createCommunityInvite.mockReturnValueOnce(pending.promise)
    const { community, wrapper, session } = mountCommunity()
    await flushPromises()
    const joining = community.join('group')
    if (reason === 'owner cleared') session.clear()
    else wrapper.unmount()
    pending.resolve({ url: 'https://t.me/+previous-owner', expiresAt: '2026-08-31T12:30:00Z' })
    await joining

    expect(mocks.openExternalLink).not.toHaveBeenCalled()
    expect(mocks.notifyHaptic).not.toHaveBeenCalled()
    if (reason === 'owner cleared') wrapper.unmount()
  })
})
