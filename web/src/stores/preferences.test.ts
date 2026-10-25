import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { UserPreferences } from '@/api/preferences'
const mocks = vi.hoisted(() => ({ get: vi.fn(), update: vi.fn() }))
vi.mock('@/api/preferences', () => ({ preferencesApi: mocks }))
import { usePreferencesStore } from './preferences'

const defaults: UserPreferences = { paygEnabled: true, paygSquads: {}, notifications: { combos: true, traffic: true, money: true, activity: true, account: true }, showReferralUsername: true, showAroundTx: false, showActivity: false, includeNodePrices: true, activeCombo: true }
describe('account preferences', () => {
  beforeEach(() => { setActivePinia(createPinia()); vi.resetAllMocks(); mocks.get.mockResolvedValue(defaults) })
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
