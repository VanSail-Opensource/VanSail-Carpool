import { defineComponent, h, type ShallowRef } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { featuresApi, type AdminAffiliateView, type AffiliateTier } from '@/api/features'
import { useAdminAffiliates } from './useAdminAffiliates'

const cache = vi.hoisted(() => ({ restoreRef: vi.fn(), restoreItems: vi.fn() }))
vi.mock('@/api/cache/restore', () => cache)
const tier: AffiliateTier = { id: 'starter', name: 'Starter', threshold: 0, enabled: true, commissionEnabled: false, commissionBps: 0, reward: { kind: 'none' } }
const config = (version: number): AdminAffiliateView => ({ version, tiers: [tier], bot: { status: 'ready', username: 'test_bot' } })
function render() {
  let state!: ReturnType<typeof useAdminAffiliates>
  const wrapper = mount(defineComponent({ setup() { state = useAdminAffiliates(); return () => h('div') } }))
  return { state, wrapper }
}
beforeEach(() => {
  cache.restoreRef.mockReset().mockReturnValue(false)
  cache.restoreItems.mockReset().mockReturnValue(false)
  vi.spyOn(featuresApi, 'getAdminAffiliates').mockResolvedValue(config(1))
  vi.spyOn(featuresApi, 'getAdminCoupons').mockResolvedValue({ items: [] })
})
afterEach(() => vi.restoreAllMocks())

describe('affiliate configuration ordering', () => {
  it('retires a cached refresh when a versioned save completes', async () => {
    cache.restoreRef.mockImplementation((_path: string, target: ShallowRef<AdminAffiliateView>) => { target.value = config(1); return true })
    cache.restoreItems.mockReturnValue(true)
    let finish!: (value: AdminAffiliateView) => void
    vi.mocked(featuresApi.getAdminAffiliates).mockReturnValueOnce(new Promise(resolve => { finish = resolve }))
    vi.spyOn(featuresApi, 'saveAdminAffiliates').mockResolvedValue(config(2))
    const { state, wrapper } = render()
    await state.save()
    finish(config(1))
    await flushPromises()
    expect(state.configuration.value?.version).toBe(2)
    expect(state.error.value).toBeNull()
    wrapper.unmount()
  })

  it('preserves edits made during save while advancing the confirmed configuration version', async () => {
    let finish!: (value: AdminAffiliateView) => void
    const save = vi.spyOn(featuresApi, 'saveAdminAffiliates').mockReturnValueOnce(new Promise(resolve => { finish = resolve }))
    const { state, wrapper } = render()
    await flushPromises()
    const pending = state.save()
    state.replace([{ ...tier, name: 'Next edit' }])
    finish(config(2))
    await pending
    expect(save).toHaveBeenCalledExactlyOnceWith(1, [tier])
    expect(state.tiers.value[0]?.name).toBe('Next edit')
    expect(state.configuration.value?.version).toBe(2)
    wrapper.unmount()
  })

  it('clears the departed owner state and ignores a late save response', async () => {
    let finish!: (value: AdminAffiliateView) => void
    const save = vi.spyOn(featuresApi, 'saveAdminAffiliates').mockReturnValueOnce(new Promise(resolve => { finish = resolve }))
    const { state, wrapper } = render()
    await flushPromises()
    const pending = state.save()
    wrapper.unmount()
    finish(config(2))
    await pending
    await state.save()
    await state.load()
    expect(state.configuration.value).toBeNull()
    expect(state.tiers.value).toEqual([])
    expect(save).toHaveBeenCalledOnce()
    expect(featuresApi.getAdminAffiliates).toHaveBeenCalledOnce()
  })
})
