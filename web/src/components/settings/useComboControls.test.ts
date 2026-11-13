import { defineComponent, reactive } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { ComboControls, EarlyActivationQuote, OperationReceipt, Purchase } from '@/api/types'

const mocks = vi.hoisted(() => ({ get: vi.fn(), quoteActivation: vi.fn(), switchSquad: vi.fn(), activate: vi.fn() }))
vi.mock('@/api/comboControls', () => ({ comboControlsApi: mocks }))
const account = reactive<{ user: { id: string } | null }>({ user: { id: 'first' } })
vi.mock('@/stores/session', () => ({ useSessionStore: () => account }))
import { useComboControls } from './useComboControls'

const at = '2026-11-13T00:00:00Z'
function purchase(owner: string, status: 'active' | 'queued'): Purchase {
  const price = { currency: 'TXB' as const, minor: '1000', display: '10.00 TXB' }
  return { id: `${owner}-${status}`, comboId: 'combo', comboName: 'Constructed plan', status, price, grossPrice: price,
    couponDiscount: { ...price, minor: '0', display: '0.00 TXB' }, couponGrantId: null, validFrom: at, validUntil: '2026-12-13T00:00:00Z',
    trafficLimitBytes: '1000', resetStrategy: 'MONTH_ROLLING', autoRenewEnabled: false, squadUuids: ['squad'], createdAt: at, updatedAt: at }
}
const controls = (owner: string): ComboControls => ({ activePurchase: purchase(owner, 'active'), queuedPurchase: purchase(owner, 'queued'),
  squads: [{ uuid: 'squad', name: 'Squad', enabled: true, canDisable: false }], mutable: true, operation: null })
const quote = (owner: string): EarlyActivationQuote => ({ currentPurchaseId: `${owner}-active`, queuedPurchaseId: `${owner}-queued`,
  eligible: true, currentValidUntil: '2026-12-13T00:00:00Z', newValidFrom: at, newValidUntil: '2026-12-14T00:00:00Z' })
let state: ReturnType<typeof useComboControls>
const Harness = defineComponent({ setup() { state = useComboControls(); return () => null } })
beforeEach(() => {
  vi.resetAllMocks(); account.user = { id: 'first' }
  mocks.get.mockImplementation(async () => controls(account.user!.id))
  mocks.quoteActivation.mockImplementation(async () => quote(account.user!.id))
})

describe('settings subscription owner and eligibility recovery', () => {
  it('retires old owner reads before quoting or restoring their purchase', async () => {
    let resolve!: (value: ComboControls) => void
    mocks.get.mockReturnValueOnce(new Promise(done => { resolve = done }))
    const wrapper = mount(Harness)
    account.user = { id: 'second' }
    expect(state.value.value).toBeNull()
    await flushPromises()
    resolve(controls('first'))
    await flushPromises()
    expect(state.value.value?.activePurchase?.id).toBe('second-active')
    expect(state.quote.value?.currentPurchaseId).toBe('second-active')
    expect(mocks.quoteActivation).toHaveBeenCalledOnce()
    wrapper.unmount()
    await state.load()
    expect(mocks.get).toHaveBeenCalledTimes(2)
  })
  it('blocks writes after failed eligibility refresh or a mismatched quote', async () => {
    const wrapper = mount(Harness)
    await flushPromises()
    mocks.get.mockRejectedValueOnce(new Error('constructed read failure'))
    await state.load()
    expect(state.blocked.value).toBe(true)
    await state.switchSquad('squad', false)
    expect(await state.activate('ACTIVATE NEXT COMBO')).toBe(false)
    expect(mocks.switchSquad).not.toHaveBeenCalled()
    expect(mocks.activate).not.toHaveBeenCalled()
    mocks.quoteActivation.mockResolvedValueOnce(quote('unrelated'))
    await state.load()
    expect(await state.activate('ACTIVATE NEXT COMBO')).toBe(false)
    expect(mocks.activate).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('does not report a retired activation as success in a replacement account', async () => {
    const wrapper = mount(Harness)
    await flushPromises()
    let resolve!: (value: OperationReceipt) => void
    mocks.activate.mockReturnValueOnce(new Promise(done => { resolve = done }))
    const activating = state.activate('ACTIVATE NEXT COMBO')
    account.user = { id: 'second' }
    await flushPromises()
    resolve({ id: 'retired-operation', kind: 'member_early_activation', status: 'queued', createdAt: at, updatedAt: at })
    expect(await activating).toBe(false)
    expect(state.command.receipt.value).toBeNull()
    expect(state.value.value?.activePurchase?.id).toBe('second-active')
    expect(mocks.get).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })
})
