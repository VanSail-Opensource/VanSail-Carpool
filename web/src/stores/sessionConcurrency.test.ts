import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '@/api/client'
import { ApiError } from '@/api/http'
import type { Session } from '@/api/types'
import { setCacheSession } from '@/api/cache/session'
import { setExperience } from '@/utils/experience'
import { useSessionStore } from './session'

vi.mock('@/api/cache/preload', () => ({ preloadSession: vi.fn(), stopSessionPreload: vi.fn() }))
vi.mock('@/router/memberViews', () => ({ preloadMemberViews: vi.fn(), stopMemberViewPreload: vi.fn() }))

const member: Session = { authenticated: true, user: {
  id: 'member-1', telegramId: '42', firstName: 'Member', lastName: '', telegramUsername: 'member', displayName: '@member',
  username: 'member', role: 'user', onboardingState: 'complete', groupJoined: true, channelJoined: true,
  policyAcceptedAt: '2026-11-12T00:00:00Z', agreementRevision: 1, recoveryReason: '',
  createdAt: '2026-11-12T00:00:00Z', updatedAt: '2026-11-12T00:00:00Z',
} }

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (error: Error) => void
  const promise = new Promise<T>((done, fail) => { resolve = done; reject = fail })
  return { promise, resolve, reject }
}

beforeEach(() => { setExperience('miniapp'); setActivePinia(createPinia()); setCacheSession(null) })
afterEach(() => { vi.restoreAllMocks(); setCacheSession(null) })

describe('session bootstrap identity boundaries', () => {
  it('retries temporary session storage failures without starting Telegram authentication', async () => {
    const getMe = vi.spyOn(api, 'getMe').mockRejectedValueOnce(new ApiError(503, { code: 'SESSION_UNAVAILABLE', message: 'Unavailable' })).mockResolvedValueOnce(member)
    const auth = vi.spyOn(api, 'authTelegram')
    const store = useSessionStore()
    await store.bootstrap()
    expect(store.status).toBe('error')
    expect(store.errorCode).toBe('SESSION_UNAVAILABLE')
    expect(auth).not.toHaveBeenCalled()
    await store.bootstrap(true)
    expect(getMe).toHaveBeenCalledTimes(2)
    expect(store.user?.id).toBe(member.user.id)
    expect(store.status).toBe('ready')
    expect(auth).not.toHaveBeenCalled()
  })

  it('makes every concurrent caller wait for the shared bootstrap', async () => {
    const response = deferred<Session>()
    const getMe = vi.spyOn(api, 'getMe').mockReturnValue(response.promise)
    const store = useSessionStore()
    const first = store.bootstrap()
    let secondFinished = false
    const second = store.bootstrap().then(() => { secondFinished = true })
    await Promise.resolve()
    expect(secondFinished).toBe(false)
    response.resolve(member)
    await Promise.all([first, second])
    expect(getMe).toHaveBeenCalledTimes(1)
    expect(store.user?.id).toBe('member-1')
  })

  it('cannot restore an identity after the session is cleared', async () => {
    const response = deferred<Session>()
    vi.spyOn(api, 'getMe').mockReturnValue(response.promise)
    const store = useSessionStore()
    const pending = store.bootstrap()
    store.clear()
    response.resolve(member)
    await pending
    expect(store.session).toBeNull()
    expect(store.status).toBe('idle')
  })

  it('ignores a superseded failure after a newer forced refresh succeeds', async () => {
    const response = deferred<Session>()
    vi.spyOn(api, 'getMe').mockReturnValueOnce(response.promise).mockResolvedValueOnce(member)
    const store = useSessionStore()
    const old = store.bootstrap()
    await store.bootstrap(true)
    response.reject(new Error('late connection failure'))
    await old
    expect(store.session).toEqual(member)
    expect(store.status).toBe('ready')
    expect(store.error).toBeNull()
  })

  it('preserves an explicitly refreshed session over an earlier bootstrap result', async () => {
    const response = deferred<Session>()
    vi.spyOn(api, 'getMe').mockReturnValue(response.promise)
    const store = useSessionStore()
    const pending = store.bootstrap()
    store.updateSession(member)
    response.resolve({ ...member, user: { ...member.user, onboardingState: 'agreement' } })
    await pending
    expect(store.onboardingComplete).toBe(true)
  })
})
