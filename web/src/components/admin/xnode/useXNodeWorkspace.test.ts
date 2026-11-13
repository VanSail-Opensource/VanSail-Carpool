import { defineComponent } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
const api = vi.hoisted(() => ({ nodes: vi.fn(), configurations: vi.fn(), snippets: vi.fn(), groups: vi.fn(), endpoints: vi.fn(), metrics: vi.fn() }))
vi.mock('@/api/xnode', () => ({ xnodeApi: api }))
import { useXNodeWorkspace } from './useXNodeWorkspace'

function mountWorkspace() {
  let state!: ReturnType<typeof useXNodeWorkspace>
  const wrapper = mount(defineComponent({ setup() { state = useXNodeWorkspace(); return () => null } }))
  return { state, wrapper }
}

describe('workspace request lifetime', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    for (const key of ['nodes', 'configurations', 'snippets', 'groups', 'endpoints'] as const) api[key].mockResolvedValue({ items: [] })
    api.metrics.mockResolvedValue({ nodes: [], fetchedAt: 'current' })
  })
  it('aborts superseded loads and rejects responses from transports ignoring cancellation', async () => {
    let resolve!: (value: unknown) => void
    api.nodes.mockReturnValueOnce(new Promise(done => { resolve = done }))
    const { state, wrapper } = mountWorkspace()
    const signal = api.nodes.mock.calls[0]![0] as AbortSignal
    await state.load()
    resolve({ items: [{ id: 'obsolete' }] }); await flushPromises()
    expect(signal.aborted).toBe(true)
    expect(state.nodes.value).toEqual([])
    wrapper.unmount(); expect(state.metrics.value).toBeNull()
  })
  it('prevents writes after a failed load', async () => {
    api.nodes.mockRejectedValue(new Error('unavailable'))
    const { state, wrapper } = mountWorkspace()
    await flushPromises()
    const action = vi.fn()
    expect(await state.perform(action)).toBe(false)
    expect(action).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('does not reload the inventory when a mutation completes after disposal', async () => {
    const { state, wrapper } = mountWorkspace()
    await flushPromises()
    let finish!: () => void
    const action = state.perform(() => new Promise<void>(resolve => { finish = resolve }))
    wrapper.unmount()
    finish()
    expect(await action).toBe(false)
    expect(api.nodes).toHaveBeenCalledTimes(1)
    expect(state.metrics.value).toBeNull()
  })
})
