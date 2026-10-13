import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { BrowserSession } from '@/api/browserAccess'
import { ApiError } from '@/api/http'
import { cacheGeneration, readResponseCache, setCacheSession, writeResponseCache } from '@/api/cache/session'
import { useSessionStore } from '@/stores/session'

const mocks = vi.hoisted(() => ({
  config: vi.fn(), session: vi.fn(), renew: vi.fn(), logout: vi.fn(), exchange: vi.fn(),
  getMe: vi.fn(), authTelegram: vi.fn(), preloadSession: vi.fn(), stopSessionPreload: vi.fn(),
}))
vi.mock('@/api/browserAccess', () => ({ browserAccessApi: mocks }))
vi.mock('@/api/client', async () => {
  const { ApiError } = await import('@/api/http')
  return { api: { getMe: mocks.getMe, authTelegram: mocks.authTelegram }, ApiError }
})
vi.mock('@/api/cache/preload', () => ({ preloadSession: mocks.preloadSession, stopSessionPreload: mocks.stopSessionPreload }))
import { useBrowserSession } from './session'

const config = { miniAppUrl: 'https://t.me/txcarpool_bot?startapp', oauthEnabled: true }
const anonymous: BrowserSession = { authenticated: false, kind: 'anonymous', miniAppUrl: config.miniAppUrl }
const admin: BrowserSession = {
  authenticated: true, kind: 'admin', expiresAt: '2026-11-12T00:00:00Z',
  user: {
    id: 'admin-1', telegramId: '42', firstName: 'Admin', lastName: '', telegramUsername: 'admin',
    username: null, role: 'admin', onboardingState: 'intro', groupJoined: false, channelJoined: false,
    policyAcceptedAt: null, agreementRevision: 0, recoveryReason: '',
    createdAt: '2026-10-13T00:00:00Z', updatedAt: '2026-10-13T00:00:00Z',
  },
}
let previousExperience: string | undefined

beforeEach(() => {
  previousExperience = document.documentElement.dataset.experience
  document.documentElement.dataset.experience = 'browser'
  setActivePinia(createPinia())
  setCacheSession(null)
  for (const mock of Object.values(mocks)) mock.mockReset()
  mocks.config.mockResolvedValue(config)
  mocks.session.mockResolvedValue(admin)
  mocks.renew.mockResolvedValue(admin)
  mocks.logout.mockResolvedValue(undefined)
})

afterEach(() => {
  setCacheSession(null)
  if (previousExperience === undefined) delete document.documentElement.dataset.experience
  else document.documentElement.dataset.experience = previousExperience
  vi.restoreAllMocks()
})

describe('isolated browser sessions', () => {
  it('deduplicates concurrent bootstrap and hydrates without Mini App authentication or preload', async () => {
    let release!: (value: typeof config) => void
    mocks.config.mockImplementation(() => new Promise(resolve => { release = resolve }))
    const browser = useBrowserSession()
    const first = browser.bootstrap()
    const second = browser.bootstrap()
    expect(browser.loading).toBe(true)
    expect(mocks.config).toHaveBeenCalledTimes(1)
    release(config)
    await Promise.all([first, second])
    expect(mocks.session).toHaveBeenCalledTimes(1)
    expect(browser.isAdmin).toBe(true)
    expect(useSessionStore().user?.id).toBe('admin-1')
    expect(mocks.authTelegram).not.toHaveBeenCalled()
    expect(mocks.preloadSession).not.toHaveBeenCalled()
  })

  it('renews expiry while preserving the mounted identity and cached draft context', async () => {
    const browser = useBrowserSession()
    await browser.bootstrap()
    writeResponseCache('draft-context', { edited: true }, cacheGeneration())
    mocks.renew.mockResolvedValue({ ...admin, expiresAt: '2026-11-13T00:00:00Z' })
    await browser.renew()
    expect(browser.state).toMatchObject({ kind: 'admin', expiresAt: '2026-11-13T00:00:00Z' })
    expect(useSessionStore().user?.id).toBe('admin-1')
    expect(readResponseCache('draft-context')).toEqual({ edited: true })
    expect(browser.expired).toBe(false)
  })

  it('keeps the existing editor identity on expiry and restores it after another login', async () => {
    const browser = useBrowserSession()
    await browser.bootstrap()
    const identity = useSessionStore().session
    mocks.renew.mockRejectedValue(new ApiError(401, { code: 'UNAUTHENTICATED', message: 'expired' }))
    mocks.session.mockResolvedValue(anonymous)
    await browser.renew()
    expect(browser.isAdmin).toBe(false)
    expect(browser.expired).toBe(true)
    expect(useSessionStore().session).toBe(identity)
    mocks.session.mockResolvedValue(admin)
    await browser.bootstrap(true)
    expect(browser.expired).toBe(false)
    expect(browser.isAdmin).toBe(true)
  })

  it('preserves authentication and cached drafts during a transient renewal failure', async () => {
    const browser = useBrowserSession()
    await browser.bootstrap()
    mocks.renew.mockRejectedValue(new ApiError(503, { code: 'BROWSER_SESSION_UNAVAILABLE', message: 'offline' }))
    await browser.renew()
    expect(browser.isAdmin).toBe(true)
    expect(browser.expired).toBe(false)
    expect(mocks.session).toHaveBeenCalledTimes(1)
  })

  it('revokes browser logout before clearing the local identity and restoring anonymous state', async () => {
    const browser = useBrowserSession()
    await browser.bootstrap()
    let revoke!: () => void
    mocks.logout.mockImplementation(() => new Promise<void>(resolve => { revoke = resolve }))
    mocks.session.mockResolvedValue(anonymous)
    const pending = browser.logout()
    expect(useSessionStore().isAdmin).toBe(true)
    revoke()
    await pending
    expect(mocks.logout).toHaveBeenCalledTimes(1)
    expect(useSessionStore().session).toBeNull()
    expect(browser.state).toEqual(anonymous)
    expect(browser.expired).toBe(false)
    expect(mocks.authTelegram).not.toHaveBeenCalled()
  })

  it('drops Mini App snapshots when the same administrator enters browser scope', async () => {
    document.documentElement.dataset.experience = 'miniapp'
    useSessionStore().updateSession(admin)
    writeResponseCache('member-only', { balance: '100' }, cacheGeneration())
    expect(mocks.preloadSession).toHaveBeenCalledTimes(1)
    document.documentElement.dataset.experience = 'browser'
    await useBrowserSession().bootstrap()
    expect(readResponseCache('member-only')).toBeUndefined()
    expect(mocks.preloadSession).toHaveBeenCalledTimes(1)
  })

  it('keeps verified member information outside the administrator identity store', async () => {
    mocks.session.mockResolvedValue({ authenticated: false, kind: 'member', miniAppUrl: config.miniAppUrl, user: { telegramId: '99', firstName: 'Member', lastName: '', telegramUsername: 'member' } })
    const browser = useBrowserSession()
    await browser.bootstrap()
    expect(browser.state?.kind).toBe('member')
    expect(browser.isAdmin).toBe(false)
    expect(useSessionStore().session).toBeNull()
  })

  it('exchanges the short-lived link once and retains its local destination', async () => {
    mocks.exchange.mockResolvedValue({ ...admin, returnTo: '/admin/users/user-1' })
    const browser = useBrowserSession()
    expect(await browser.exchange('one-use-token')).toBe('/admin/users/user-1')
    expect(mocks.exchange).toHaveBeenCalledExactlyOnceWith('one-use-token')
    expect(browser.isAdmin).toBe(true)
    expect(mocks.authTelegram).not.toHaveBeenCalled()
  })
})
