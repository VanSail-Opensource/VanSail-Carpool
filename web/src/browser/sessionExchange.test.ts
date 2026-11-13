import { createPinia, setActivePinia } from 'pinia'
import { flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { BrowserConfig, BrowserSession } from '@/api/browserAccess'
import { useSessionStore } from '@/stores/session'

const mocks = vi.hoisted(() => ({ config: vi.fn(), session: vi.fn(), exchange: vi.fn(), logout: vi.fn() }))
vi.mock('@/api/browserAccess', () => ({ browserAccessApi: mocks }))
vi.mock('@/api/cache/preload', () => ({ preloadSession: vi.fn(), stopSessionPreload: vi.fn() }))
import { useBrowserSession } from './session'

const config = { miniAppUrl: '' }
const anonymous: BrowserSession = { authenticated: false, kind: 'anonymous', miniAppUrl: '' }
const admin: BrowserSession = { authenticated: true, kind: 'admin', expiresAt: '2026-11-13T00:00:00Z', user: {
  id: 'admin', telegramId: '42', firstName: 'Admin', lastName: '', telegramUsername: 'admin', displayName: 'Admin', username: null,
  role: 'admin', onboardingState: 'complete', groupJoined: false, channelJoined: false, policyAcceptedAt: null,
  agreementRevision: 1, recoveryReason: '', createdAt: '2026-11-12T00:00:00Z', updatedAt: '2026-11-12T00:00:00Z',
} }
function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>(accept => { resolve = accept })
  return { promise, resolve }
}
let experience: string | undefined
beforeEach(() => {
  experience = document.documentElement.dataset.experience
  document.documentElement.dataset.experience = 'browser'
  setActivePinia(createPinia())
  vi.resetAllMocks()
  mocks.config.mockResolvedValue(config)
  mocks.session.mockResolvedValue(anonymous)
  mocks.logout.mockResolvedValue(undefined)
})
afterEach(() => {
  useSessionStore().clear()
  if (experience === undefined) delete document.documentElement.dataset.experience
  else document.documentElement.dataset.experience = experience
})

describe('browser exchange ownership', () => {
  it('does not consume a link after disposal during configuration loading', async () => {
    const pending = deferred<BrowserConfig>(), controller = new AbortController()
    mocks.config.mockReturnValueOnce(pending.promise)
    const browser = useBrowserSession()
    const result = browser.exchange('private-token', controller.signal).catch(error => error)
    controller.abort()
    pending.resolve(config)
    expect(await result).toMatchObject({ name: 'AbortError' })
    expect(mocks.exchange).not.toHaveBeenCalled()
    expect(browser.isAdmin).toBe(false)
  })

  it('does not restore an administrator after logout supersedes a pending exchange', async () => {
    const pending = deferred<BrowserSession>()
    mocks.exchange.mockReturnValueOnce(pending.promise)
    const browser = useBrowserSession()
    const result = browser.exchange('private-token').catch(error => error)
    await flushPromises()
    await browser.logout()
    pending.resolve(admin)
    expect(await result).toMatchObject({ name: 'AbortError' })
    expect(browser.state).toEqual(anonymous)
    expect(useSessionStore().user).toBeNull()
  })

  it('rejects an earlier exchange after a newer administrator signs in', async () => {
    const pending = deferred<BrowserSession>()
    mocks.exchange.mockReturnValueOnce(pending.promise).mockResolvedValueOnce({ ...admin, user: { ...admin.user, id: 'new-admin' } })
    const browser = useBrowserSession()
    const first = browser.exchange('first-token').catch(error => error)
    await flushPromises()
    await browser.exchange('second-token')
    pending.resolve(admin)
    expect(await first).toMatchObject({ name: 'AbortError' })
    expect(useSessionStore().user?.id).toBe('new-admin')
  })
})
