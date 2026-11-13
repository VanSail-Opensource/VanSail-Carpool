import { defineComponent, h, reactive } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '@/api/client'
import { ipLookupApi, type IPLookupAdminSettings } from '@/api/ipLookup'
import { useAdminIPLookup } from './useAdminIPLookup'

const session = reactive({ user: { id: 'first-owner' } })
vi.mock('@/stores/session', () => ({ useSessionStore: () => session }))
const configuration = (): IPLookupAdminSettings => ({ enabled: true, lookupFeeTxb: '1.00', refreshFeeTxb: '2.00', providers: [], geolocationOrder: [], configured: {}, comboQuotas: {} })
function render() {
  let state!: ReturnType<typeof useAdminIPLookup>
  const wrapper = mount(defineComponent({ setup() { state = useAdminIPLookup(); return () => h('div') } }))
  return { state, wrapper }
}
beforeEach(() => {
  session.user = { id: 'first-owner' }
  vi.spyOn(api, 'getAdminResource').mockResolvedValue({ items: [] })
  vi.spyOn(ipLookupApi, 'settings').mockResolvedValue(configuration())
})
afterEach(() => vi.restoreAllMocks())

describe('IP lookup administrator settings lifetime', () => {
  it('does not write empty defaults after settings failed to load', async () => {
    vi.mocked(ipLookupApi.settings).mockRejectedValueOnce(new Error('unavailable'))
    const save = vi.spyOn(ipLookupApi, 'saveSettings').mockResolvedValue(configuration())
    const { state, wrapper } = render()
    await flushPromises()
    expect(state.loaded.value).toBe(false)
    await state.save()
    expect(save).not.toHaveBeenCalled()
    await state.load()
    expect(state.loaded.value).toBe(true)
    await state.save()
    expect(save).toHaveBeenCalledOnce()
    wrapper.unmount()
  })
  it('retains readable settings but prevents writes after a failed refresh', async () => {
    const save = vi.spyOn(ipLookupApi, 'saveSettings').mockResolvedValue(configuration())
    const { state, wrapper } = render()
    await flushPromises()
    expect(state.loaded.value).toBe(true)
    vi.mocked(ipLookupApi.settings).mockRejectedValueOnce(new Error('unavailable'))
    await state.load()
    expect(state.settings.lookupFeeTxb).toBe('1.00')
    expect(state.loaded.value).toBe(false)
    await state.save()
    expect(save).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('sends an isolated reviewed snapshot and blocks concurrent reads and writes', async () => {
    let finish!: (value: IPLookupAdminSettings) => void
    const save = vi.spyOn(ipLookupApi, 'saveSettings').mockReturnValue(new Promise(resolve => { finish = resolve }))
    const { state, wrapper } = render()
    await flushPromises()
    state.settings.credentials!.cloudflare_radar = { value: 'reviewed-token', clear: false }
    const pending = state.save()
    state.settings.credentials!.cloudflare_radar.value = 'later-token'
    await state.load()
    await state.save()
    expect(save).toHaveBeenCalledOnce()
    expect(save.mock.calls[0]![0].credentials!.cloudflare_radar.value).toBe('reviewed-token')
    expect(ipLookupApi.settings).toHaveBeenCalledOnce()
    finish(configuration())
    await pending
    expect(state.saved.value).toBe(true)
    wrapper.unmount()
  })

  it('clears secrets and retires a save when the account changes', async () => {
    let finish!: (value: IPLookupAdminSettings) => void
    vi.spyOn(ipLookupApi, 'saveSettings').mockReturnValue(new Promise(resolve => { finish = resolve }))
    const { state, wrapper } = render()
    await flushPromises()
    state.settings.credentials!.cloudflare_radar = { value: 'first-owner-secret', clear: false }
    const pending = state.save()
    session.user = { id: 'second-owner' }
    await flushPromises()
    finish({ ...configuration(), lookupFeeTxb: '99.00' })
    await pending
    expect(state.settings.lookupFeeTxb).toBe('1.00')
    expect(state.settings.credentials!.cloudflare_radar.value).toBe('')
    expect(state.saved.value).toBe(false)
    wrapper.unmount()
  })

  it('clears credentials on disposal and rejects later submissions', async () => {
    const save = vi.spyOn(ipLookupApi, 'saveSettings').mockResolvedValue(configuration())
    const { state, wrapper } = render()
    await flushPromises()
    state.settings.credentials!.cloudflare_radar = { value: 'secret', clear: false }
    wrapper.unmount()
    await state.save()
    await state.load()
    expect(state.settings.credentials).toEqual({})
    expect(save).not.toHaveBeenCalled()
    expect(ipLookupApi.settings).toHaveBeenCalledOnce()
  })
})
