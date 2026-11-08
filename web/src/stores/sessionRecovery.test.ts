import { createPinia, setActivePinia } from 'pinia'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, ApiError } from '@/api/client'
import { setExperience } from '@/utils/experience'
import { useSessionStore } from './session'

vi.mock('@/utils/telegram', () => ({ getTelegramInitData: async () => 'verified-fixture', isTelegramWebAppDetected: () => true }))
vi.mock('@/api/cache/preload', () => ({ preloadSession: vi.fn(), stopSessionPreload: vi.fn() }))

afterEach(() => vi.restoreAllMocks())

describe('partial cleanup session boundary', () => {
  it('retains verified recovery classification without authenticating, then clears it on retry', async () => {
    setExperience('miniapp')
    setActivePinia(createPinia())
    vi.spyOn(api, 'getMe').mockRejectedValue(new ApiError(401, { code: 'UNAUTHENTICATED', message: '' }))
    const exchange = vi.spyOn(api, 'authTelegram').mockRejectedValue(new ApiError(409, { code: 'ACCOUNT_RECOVERY_PENDING', message: '' }))
    const store = useSessionStore()
    await store.bootstrap()
    expect(store.status).toBe('error')
    expect(store.errorCode).toBe('ACCOUNT_RECOVERY_PENDING')
    expect(store.isAuthenticated).toBe(false)
    expect(store.session).toBeNull()
    exchange.mockRejectedValue(new Error('temporary connection failure'))
    await store.bootstrap(true)
    expect(store.errorCode).toBeNull()
    expect(store.isAuthenticated).toBe(false)
    store.clear()
    expect(store.errorCode).toBeNull()
  })
})
