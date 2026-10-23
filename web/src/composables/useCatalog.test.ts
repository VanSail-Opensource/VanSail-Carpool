import { effectScope, nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { Catalog, SquadProduct } from '@/api/types'

const { getCatalog, getBalance, getCouponWallet, quotePurchase, createPurchase } = vi.hoisted(() => ({
  getCatalog: vi.fn(),
  getBalance: vi.fn(),
  getCouponWallet: vi.fn(),
  quotePurchase: vi.fn(),
  createPurchase: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  api: { getCatalog, getBalance, quotePurchase, createPurchase },
  ApiError: class ApiError extends Error {},
}))

vi.mock('@/api/features', () => ({
  featuresApi: { getCouponWallet },
}))

vi.mock('@/stores/session', () => ({
  useSessionStore: () => ({ user: { id: 'user-1' } }),
}))

import { useCatalog } from './useCatalog'

const money = (minor: string) => ({ currency: 'TXB' as const, minor, display: `${Number(minor) / 100} TXB` })
const squad = (id: string): SquadProduct => ({
  id,
  groupId: `00000000-0000-4000-8000-${id.padStart(12, '0')}`,
  name: `Squad ${id}`,
  description: '',
  profile: null,
  price: money('100'),
  visible: true,
  upstreamPresent: true,
  stockHeldByCurrentUser: false,
  activationRequired: false,
  accessibleNodes: [],
  createdAt: '2026-08-08T00:00:00Z',
  updatedAt: '2026-08-08T00:00:00Z',
})

describe('catalog squad selection', () => {
  beforeEach(() => setActivePinia(createPinia()))
  afterEach(() => {
    sessionStorage.clear()
    vi.clearAllMocks()
  })

  it('disables included squads and prunes newly included paid add-ons after a combo change', async () => {
    const alpha = squad('1')
    const beta = squad('2')
    const catalog = {
      addons: [alpha, beta],
      nodes: [],
      combos: [
        { id: 'combo-a', name: 'A', description: '', price: money('1000'), validityDays: 30, trafficLimitBytes: '1', resetStrategy: 'MONTH_ROLLING', includedSquads: [alpha], active: true, rolloverMinRemainingBps: 0, createdAt: '2026-08-08T00:00:00Z', updatedAt: '2026-08-08T00:00:00Z' },
        { id: 'combo-b', name: 'B', description: '', price: money('1000'), validityDays: 30, trafficLimitBytes: '1', resetStrategy: 'MONTH_ROLLING', includedSquads: [beta], active: true, rolloverMinRemainingBps: 0, createdAt: '2026-08-08T00:00:00Z', updatedAt: '2026-08-08T00:00:00Z' },
      ],
    } satisfies Catalog
    getCatalog.mockResolvedValue(catalog)
    getBalance.mockResolvedValue({ balance: money('5000'), paymentMethods: [] })
    getCouponWallet.mockResolvedValue({ items: [] })
    const scope = effectScope()
    const state = scope.run(() => useCatalog())!
    await state.load()

    state.toggleSquad(alpha.id)
    expect(state.selectedSquadIds.value).toEqual([])
    state.toggleSquad(beta.id)
    expect(state.selectedSquadIds.value).toEqual([beta.id])

    state.selectCombo('combo-b')
    expect(state.selectedSquadIds.value).toEqual([])
    expect(state.includedSquadIds.value).toEqual([beta.id])
    scope.stop()
  })

  it('does not select a full paid add-on', async () => {
    const full = { ...squad('1'), stockRemaining: 0 }
    const catalog = {
      addons: [full],
      nodes: [],
      combos: [
        { id: 'combo-a', name: 'A', description: '', price: money('1000'), validityDays: 30, trafficLimitBytes: '1', resetStrategy: 'MONTH_ROLLING', includedSquads: [], active: true, rolloverMinRemainingBps: 0, createdAt: '2026-08-08T00:00:00Z', updatedAt: '2026-08-08T00:00:00Z' },
      ],
    } satisfies Catalog
    getCatalog.mockResolvedValue(catalog)
    getBalance.mockResolvedValue({ balance: money('5000'), paymentMethods: [] })
    getCouponWallet.mockResolvedValue({ items: [] })
    const scope = effectScope()
    const state = scope.run(() => useCatalog())!
    await state.load()

    state.toggleSquad(full.id)

    expect(state.selectedSquadIds.value).toEqual([])
    scope.stop()
  })

  it('reuses one purchase idempotency key after an ambiguous failed attempt', async () => {
    const addon = squad('1')
    const catalog = {
      addons: [addon],
      nodes: [],
      combos: [
        { id: 'combo-a', name: 'A', description: '', price: money('1000'), validityDays: 30, trafficLimitBytes: '1', resetStrategy: 'MONTH_ROLLING', includedSquads: [], active: true, rolloverMinRemainingBps: 0, createdAt: '2026-08-08T00:00:00Z', updatedAt: '2026-08-08T00:00:00Z' },
      ],
    } satisfies Catalog
    getCatalog.mockResolvedValue(catalog)
    getBalance.mockResolvedValue({ balance: money('5000'), paymentMethods: [] })
    getCouponWallet.mockResolvedValue({ items: [] })
    quotePurchase.mockResolvedValue({
      comboId: 'combo-a',
      comboName: 'A',
      grossPrice: money('1000'),
      discount: money('0'),
      netPrice: money('1000'),
      effectiveAt: '2026-08-08T00:00:00Z',
      expiresAt: '2026-09-07T00:00:00Z',
      queued: false,
      addonSquadUuids: [],
      accessibleNodes: [],
    })
    createPurchase
      .mockRejectedValueOnce(new Error('connection closed after commit'))
      .mockResolvedValueOnce({ id: 'purchase-1' })

    const scope = effectScope()
    const state = scope.run(() => useCatalog())!
    await state.load()
    state.toggleSquad(addon.id)
    expect(await state.confirmPurchase()).toBe(false)
    expect(await state.confirmPurchase()).toBe(true)
    await nextTick()
    expect(createPurchase).toHaveBeenCalledTimes(2)
    expect(createPurchase.mock.calls[0]?.[3]).toBeTruthy()
    expect(createPurchase.mock.calls[1]?.[3]).toBe(createPurchase.mock.calls[0]?.[3])
    expect(state.quote.value?.netPrice.minor).toBe('1000')
    expect(sessionStorage.getItem('txc-catalog-draft:user-1')).toBeNull()
    expect(await state.confirmPurchase()).toBe(false)
    expect(createPurchase).toHaveBeenCalledTimes(2)
    scope.stop()
  })
})
