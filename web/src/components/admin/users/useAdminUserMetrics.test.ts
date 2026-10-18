import { defineComponent, shallowRef, type MaybeRefOrGetter } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { XNodeProfileMetrics } from '@/api/adminOperations'
const mocks = vi.hoisted(() => ({ getUserMetrics: vi.fn() }))
vi.mock('@/api/adminOperations', () => ({ adminOperationsApi: mocks }))
import { useAdminUserMetrics } from './useAdminUserMetrics'

const metrics: XNodeProfileMetrics = { connections: 2, uploadBytesPerSecond: 100, downloadBytesPerSecond: 200, incomplete: false, limited: false, reasons: [], fetchedAt: '2026-10-18T00:00:00Z', nodes: [], usage: [], usedTrafficBytes: '9007199254740993', lifetimeTrafficBytes: '90071992547409930' }
function mountMetrics(id: MaybeRefOrGetter<string>) {
  let state!: ReturnType<typeof useAdminUserMetrics>
  const wrapper = mount(defineComponent({ setup() { state = useAdminUserMetrics(id); return () => null } }))
  return { state, wrapper }
}
describe('native profile observation scope', () => {
  beforeEach(() => vi.resetAllMocks())
  it('aborts an old profile and rejects its late result after navigation', async () => {
    let resolve!: (value: XNodeProfileMetrics) => void
    mocks.getUserMetrics.mockReturnValueOnce(new Promise<XNodeProfileMetrics>(done => { resolve = done })).mockResolvedValueOnce(metrics)
    const id = shallowRef('old')
    const { state, wrapper } = mountMetrics(id)
    const signal = mocks.getUserMetrics.mock.calls[0]![1] as AbortSignal
    id.value = 'current'; await flushPromises()
    resolve({ ...metrics, usedTrafficBytes: '1' }); await flushPromises()
    expect(signal.aborted).toBe(true)
    expect(state.metrics.value?.usedTrafficBytes).toBe('9007199254740993')
    wrapper.unmount(); expect(state.metrics.value).toBeNull(); expect(state.samples.value).toEqual({})
  })
  it('retires pending responses and chart samples on logout', async () => {
    let resolve!: (value: XNodeProfileMetrics) => void
    mocks.getUserMetrics.mockReturnValue(new Promise<XNodeProfileMetrics>(done => { resolve = done }))
    const { state, wrapper } = mountMetrics('member')
    window.dispatchEvent(new Event('txc:session-cleared'))
    resolve(metrics); await flushPromises()
    expect(state.metrics.value).toBeNull(); expect(state.samples.value).toEqual({})
    wrapper.unmount()
  })
})
