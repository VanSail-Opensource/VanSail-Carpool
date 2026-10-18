import { defineComponent } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { USDTConfig } from '@/api/usdt'
const mocks = vi.hoisted(() => ({ config: vi.fn(), status: vi.fn(), review: vi.fn(), save: vi.fn() }))
vi.mock('@/api/usdt', () => ({ usdtApi: mocks }))
import { useUSDTAdmin } from './useUSDTAdmin'

const config: USDTConfig = { mode: 'both', enabled: false, name: 'VanSail', rate: '', expiryMinutes: 30, concurrency: 1, networks: [] }
function fixture() {
  let state!: ReturnType<typeof useUSDTAdmin>
  const wrapper = mount(defineComponent({ setup() { state = useUSDTAdmin(); return () => null } }))
  return { wrapper, state }
}
describe('stablecoin setting load boundaries', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    mocks.status.mockResolvedValue({ networks: [] })
    mocks.review.mockResolvedValue({ items: [], nextCursor: '' })
    mocks.config.mockResolvedValue(config)
  })
  it('aborts the superseded load and ignores its response even if transport ignores cancellation', async () => {
    let resolve!: (value: USDTConfig) => void
    mocks.config.mockReturnValueOnce(new Promise<USDTConfig>(done => { resolve = done }))
    const { wrapper, state } = fixture()
    const oldSignal = mocks.config.mock.calls[0]![0] as AbortSignal
    await state.load()
    expect(oldSignal.aborted).toBe(true)
    resolve({ ...config, name: 'Obsolete draft' }); await flushPromises()
    expect(state.draft.value?.name).toBe('VanSail')
    wrapper.unmount()
  })
  it('blocks saving after a failed configuration load until a successful retry', async () => {
    mocks.config.mockRejectedValueOnce(new Error('unavailable'))
    const { wrapper, state } = fixture()
    await flushPromises(); await state.save()
    expect(state.loaded.value).toBe(false)
    expect(mocks.save).not.toHaveBeenCalled()
    await state.load()
    expect(state.loaded.value).toBe(true)
    expect(state.draft.value).toEqual(config)
    wrapper.unmount()
  })
  it('aborts disposal and never installs a late draft', async () => {
    let resolve!: (value: USDTConfig) => void
    mocks.config.mockReturnValueOnce(new Promise<USDTConfig>(done => { resolve = done }))
    const { wrapper, state } = fixture()
    const signal = mocks.config.mock.calls[0]![0] as AbortSignal
    wrapper.unmount(); resolve(config); await flushPromises()
    expect(signal.aborted).toBe(true)
    expect(state.draft.value).toBeNull()
  })
})
