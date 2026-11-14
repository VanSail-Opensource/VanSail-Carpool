import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { UserPreferences } from '@/api/preferences'
const mocks = vi.hoisted(() => ({ get: vi.fn(), update: vi.fn(), restoreRef: vi.fn() }))
vi.mock('@/api/preferences', () => ({ preferencesApi: mocks }))
vi.mock('@/api/cache/restore', () => ({ restoreRef: mocks.restoreRef }))
import { usePreferencesStore } from './preferences'

const defaults: UserPreferences = { paygEnabled: true, paygSquads: {}, notifications: { combos: true, traffic: true, money: true, activity: true, account: true }, showReferralUsername: true, showAroundTx: false, showActivity: false, includeNodePrices: true, activeCombo: true }
describe('account preferences', () => {
  beforeEach(() => { setActivePinia(createPinia()); vi.resetAllMocks(); mocks.get.mockResolvedValue(defaults) })
  it('restores display preferences without granting cached navigation or mutation rights', async () => {
    mocks.restoreRef.mockImplementationOnce((_path, target) => { target.value = { ...defaults, showActivity: true }; return true })
    let resolve!: (value: UserPreferences) => void
    mocks.get.mockReturnValueOnce(new Promise(done => { resolve = done }))
    const store = usePreferencesStore(); store.bind('user-1')
    expect(store.value?.showActivity).toBe(true)
    expect(store.showActivity).toBe(false)
    expect(store.blocked).toBe(true)
    await store.save({ includeNodePrices: false })
    expect(mocks.update).not.toHaveBeenCalled()
    resolve({ ...defaults, showActivity: true })
    await Promise.resolve(); await Promise.resolve()
    expect(store.showActivity).toBe(true)
    expect(store.blocked).toBe(false)
  })
  it('keeps the confirmed setting after a failed partial save', async () => {
    const store = usePreferencesStore(); store.bind('user-1'); await store.refresh()
    mocks.update.mockRejectedValue(new Error('offline'))
    await store.save({ includeNodePrices: false })
    expect(store.includeNodePrices).toBe(true)
    expect(store.error).toBeTruthy()
  })
  it('drops an old account response after switching users', async () => {
    const store = usePreferencesStore(); store.bind('user-1'); await store.refresh()
    let resolve!: (value: UserPreferences) => void
    mocks.update.mockReturnValue(new Promise<UserPreferences>(done => { resolve = done }))
    const save = store.save({ showActivity: true })
    store.bind('user-2'); await store.refresh()
    resolve({ ...defaults, showActivity: true }); await save
    expect(store.showActivity).toBe(false)
  })
})
