import { afterEach, describe, expect, it, vi } from 'vitest'
import { api } from '@/api/client'
import type { Catalog, Purchase, PurchaseAddonQuote } from '@/api/types'
import { useSquadAddition } from './useSquadAddition'

afterEach(() => vi.restoreAllMocks())

describe('squad addition delivery', () => {
  it('reuses the original request key after a lost purchase reply', async () => {
    vi.spyOn(api, 'getCatalog').mockResolvedValue({ combos: [], nodes: [], addons: [{ id: 'squad', groupId: 'squad', name: 'Native squad', visible: true, upstreamPresent: true, stockRemaining: null, activationRequired: false }] } as unknown as Catalog)
    vi.spyOn(api, 'quotePurchaseAddons').mockResolvedValue({ purchaseId: 'purchase', addonSquadUuids: ['squad'], expiresAt: '2030-01-01T00:00:00Z', price: { currency: 'TXB', minor: '100', display: '1 TXB' } } as PurchaseAddonQuote)
    const add = vi.spyOn(api, 'addPurchaseAddons').mockRejectedValueOnce(new Error('reply lost')).mockResolvedValue({ id: 'purchase', status: 'active' } as Purchase)
    const controller = useSquadAddition({ purchaseId: () => 'purchase', activeSquadUuids: () => [] })
    await controller.load()
    controller.toggleSquad('squad')
    await controller.refreshQuote()
    expect(await controller.confirmPurchase({})).toBe(false)
    expect(await controller.confirmPurchase({})).toBe(true)
    expect(add).toHaveBeenCalledTimes(2)
    expect(add.mock.calls[0]?.[2]).toBeTruthy()
    expect(add.mock.calls[1]?.[2]).toBe(add.mock.calls[0]?.[2])
    expect(controller.purchase.value?.id).toBe('purchase')
  })
})
