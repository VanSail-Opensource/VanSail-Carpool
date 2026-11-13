import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { defineComponent, h } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Session } from '@/api/types'
import { useSessionStore } from '@/stores/session'
import { useCommunityAccess } from './useCommunityAccess'

const mocks = vi.hoisted(() => ({ getCommunityAccess: vi.fn(), getBalance: vi.fn() }))
vi.mock('@/api/client', async () => ({ api: mocks, ApiError: (await import('@/api/http')).ApiError }))
vi.mock('@/api/cache/preload', () => ({ preloadSession: vi.fn(), stopSessionPreload: vi.fn() }))

function owner(id = 'owner-1'): Session {
  return { authenticated: true, user: { id, telegramId: '42', firstName: 'Member', lastName: '', telegramUsername: '', displayName: 'Member', username: null, role: 'user', onboardingState: 'complete', groupJoined: false, channelJoined: false, policyAcceptedAt: null, agreementRevision: 1, recoveryReason: '', createdAt: '2026-11-13T00:00:00Z', updatedAt: '2026-11-13T00:00:00Z' } }
}
function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason: Error) => void
  const promise = new Promise<T>((done, fail) => { resolve = done; reject = fail })
  return { promise, resolve, reject }
}
function setup(authenticated = true) {
  const pinia = createPinia()
  const session = useSessionStore(pinia)
  if (authenticated) session.session = owner()
  let access!: ReturnType<typeof useCommunityAccess>
  const wrapper = mount(defineComponent({ setup() { access = useCommunityAccess(); return () => h('div') } }), { global: { plugins: [pinia] } })
  return { session, access, wrapper }
}

describe('community access owner boundaries', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    mocks.getCommunityAccess.mockResolvedValue({ activeCombo: false })
    mocks.getBalance.mockResolvedValue({ balance: { minor: '0' } })
  })

  it('does not request private access data without an owner', async () => {
    const { access, wrapper } = setup(false)
    await access.refresh()
    expect(mocks.getCommunityAccess).not.toHaveBeenCalled()
    expect(mocks.getBalance).not.toHaveBeenCalled()
    expect(access.loading.value).toBe(false)
    expect(access.hasAccess.value).toBe(false)
    wrapper.unmount()
  })

  it('shares pending reads but immediately refreshes after the owner changes', async () => {
    const combo = deferred<{ activeCombo: boolean }>()
    const balance = deferred<{ balance: { minor: string } }>()
    mocks.getCommunityAccess.mockReturnValueOnce(combo.promise)
    mocks.getBalance.mockReturnValueOnce(balance.promise)
    const { session, access, wrapper } = setup()
    const pending = access.refresh()
    expect(access.refresh()).toBe(pending)
    session.session = owner('owner-2')
    await flushPromises()
    expect(mocks.getCommunityAccess).toHaveBeenCalledTimes(2)
    expect(mocks.getBalance).toHaveBeenCalledTimes(2)
    combo.resolve({ activeCombo: true })
    balance.resolve({ balance: { minor: '9007199254740993' } })
    await pending
    expect(access.activeCombo.value).toBe(false)
    expect(access.positiveBalance.value).toBe(false)
    expect(access.hasAccess.value).toBe(false)
    wrapper.unmount()
  })

  it('clears confirmed access at logout and ignores delayed refresh replies', async () => {
    mocks.getCommunityAccess.mockResolvedValueOnce({ activeCombo: true })
    const { session, access, wrapper } = setup()
    await flushPromises()
    expect(access.hasAccess.value).toBe(true)
    const combo = deferred<{ activeCombo: boolean }>()
    mocks.getCommunityAccess.mockReturnValueOnce(combo.promise)
    const pending = access.refresh()
    session.clear()
    expect(access.hasAccess.value).toBe(false)
    expect(access.loading.value).toBe(false)
    combo.resolve({ activeCombo: true })
    await pending
    expect(access.hasAccess.value).toBe(false)
    wrapper.unmount()
  })

  it('keeps a newer owner access grant after an older read fails', async () => {
    const combo = deferred<{ activeCombo: boolean }>()
    mocks.getCommunityAccess.mockReturnValueOnce(combo.promise).mockResolvedValueOnce({ activeCombo: true })
    const { session, access, wrapper } = setup()
    session.session = owner('owner-2')
    await flushPromises()
    combo.reject(new Error('Old owner failure'))
    await flushPromises()
    expect(access.activeCombo.value).toBe(true)
    expect(access.loading.value).toBe(false)
    wrapper.unmount()
  })

  it('disposes retained state and does not restart reads after unmount', async () => {
    const combo = deferred<{ activeCombo: boolean }>()
    mocks.getCommunityAccess.mockReturnValueOnce(combo.promise)
    const { access, wrapper } = setup()
    wrapper.unmount()
    combo.resolve({ activeCombo: true })
    await flushPromises()
    await access.refresh()
    expect(access.hasAccess.value).toBe(false)
    expect(mocks.getCommunityAccess).toHaveBeenCalledTimes(1)
  })
})
