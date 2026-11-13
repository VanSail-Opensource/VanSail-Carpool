import { effectScope, shallowRef } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { AutoRenewal } from '@/api/types'

const { getAutoRenewal, setAutoRenewal } = vi.hoisted(() => ({
  getAutoRenewal: vi.fn(),
  setAutoRenewal: vi.fn(),
}))

vi.mock('@/api/client', () => ({ api: { getAutoRenewal, setAutoRenewal } }))

import { useAutoRenewal } from './useAutoRenewal'

const ineligibleRenewal: AutoRenewal = {
  purchaseId: 'purchase-1',
  enabled: false,
  canEnable: false,
  ineligibleReason: 'INSUFFICIENT_BALANCE',
  grossPrice: { currency: 'TXB', minor: '1000', display: '10.00 TXB' },
  discount: { currency: 'TXB', minor: '0', display: '0.00 TXB' },
  netPrice: { currency: 'TXB', minor: '1000', display: '10.00 TXB' },
  scheduledAt: '2026-08-14T00:00:00Z',
  nextCycleEndsAt: '2026-09-13T00:00:00Z',
}

describe('useAutoRenewal', () => {
  beforeEach(() => { getAutoRenewal.mockReset(); setAutoRenewal.mockReset() })
  it('keeps an ineligible automatic-renewal toggle disabled', async () => {
    getAutoRenewal.mockResolvedValue(ineligibleRenewal)
    const purchaseID = shallowRef('purchase-1')
    const scope = effectScope()
    const state = scope.run(() => useAutoRenewal(() => purchaseID.value, () => false))!

    await state.load()

    expect(state.canEnable.value).toBe(false)
    expect(state.ineligibleReason.value).toBe('INSUFFICIENT_BALANCE')
    expect(await state.setEnabled(true)).toBe(false)
    expect(setAutoRenewal).not.toHaveBeenCalled()
    scope.stop()
  })

  it('ignores a completed update after the purchase changes', async () => {
    let finish!: (value: AutoRenewal) => void
    setAutoRenewal.mockReturnValue(new Promise<AutoRenewal>(resolve => { finish = resolve }))
    const purchaseID = shallowRef('purchase-1')
    const scope = effectScope()
    const state = scope.run(() => useAutoRenewal(() => purchaseID.value, () => true))!
    const pending = state.setEnabled(false)
    purchaseID.value = 'purchase-2'
    state.reset()
    finish(ineligibleRenewal)
    expect(await pending).toBe(false)
    expect(state.renewal.value).toBeNull()
    expect(state.enabled.value).toBe(true)
    expect(state.updating.value).toBe(false)
    scope.stop()
  })

  it('does not let an earlier read undo a confirmed update', async () => {
    let finish!: (value: AutoRenewal) => void
    getAutoRenewal.mockReturnValue(new Promise<AutoRenewal>(resolve => { finish = resolve }))
    setAutoRenewal.mockResolvedValue(ineligibleRenewal)
    const scope = effectScope()
    const state = scope.run(() => useAutoRenewal(() => 'purchase-1', () => true))!
    const pending = state.load()
    expect(await state.setEnabled(false)).toBe(true)
    finish({ ...ineligibleRenewal, enabled: true })
    await pending
    expect(state.enabled.value).toBe(false)
    expect(state.loading.value).toBe(false)
    scope.stop()
  })
})
