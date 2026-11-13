import { defineComponent, shallowRef } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { WorkspaceTab } from './workspaceTabs'
const api = vi.hoisted(() => ({ nodes: vi.fn(), configurations: vi.fn(), snippets: vi.fn(), groups: vi.fn(), endpoints: vi.fn(), metrics: vi.fn() }))
vi.mock('@/api/xnode', () => ({ xnodeApi: api }))
import { useXNodeWorkspace } from './useXNodeWorkspace'

function mountWorkspace(initial: WorkspaceTab = 'nodes') {
  const tab = shallowRef<WorkspaceTab>(initial)
  let state!: ReturnType<typeof useXNodeWorkspace>
  const wrapper = mount(defineComponent({ setup() { state = useXNodeWorkspace(() => tab.value); return () => null } }))
  return { state, wrapper, tab }
}

describe('workspace request lifetime', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
    for (const key of ['nodes', 'configurations', 'snippets', 'groups', 'endpoints'] as const) api[key].mockResolvedValue({ items: [] })
    api.metrics.mockResolvedValue({ nodes: [], fetchedAt: 'current' })
  })
  afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks() })
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
  it('keeps inventory and recovery writes available when optional metrics fail', async () => {
    api.metrics.mockRejectedValue(new Error('telemetry offline'))
    const { state, wrapper } = mountWorkspace()
    await flushPromises()
    expect(state.loading.value).toBe(false)
    expect(state.error.value).toBeNull()
    expect(state.metricError.value).toBeTruthy()
    expect(api.groups).not.toHaveBeenCalled()
    expect(api.snippets).not.toHaveBeenCalled()
    const action = vi.fn().mockResolvedValue(undefined)
    expect(await state.perform(action)).toBe(true)
    expect(action).toHaveBeenCalledOnce()
    wrapper.unmount()
  })
  it('does not wait for slow metrics before permitting an inventory action', async () => {
    api.metrics.mockImplementation(() => new Promise(() => undefined))
    const { state, wrapper } = mountWorkspace()
    await flushPromises()
    expect(state.loading.value).toBe(false)
    expect(await state.perform(vi.fn().mockResolvedValue(undefined))).toBe(true)
    wrapper.unmount()
  })
  it('lets independent settings load after another tab fails and fetches only dependencies', async () => {
    api.groups.mockRejectedValue(new Error('groups offline'))
    const { state, wrapper, tab } = mountWorkspace('groups')
    await flushPromises()
    expect(state.error.value).toBeTruthy()
    expect(api.metrics).not.toHaveBeenCalled()
    tab.value = 'global'
    await flushPromises()
    expect(state.error.value).toBeNull()
    expect(state.loading.value).toBe(false)
    expect(api.nodes).toHaveBeenCalledTimes(1)
    tab.value = 'configurations'
    await flushPromises()
    expect(state.error.value).toBeNull()
    expect(api.configurations).toHaveBeenCalledTimes(2)
    expect(api.snippets).toHaveBeenCalledOnce()
    expect(api.groups).toHaveBeenCalledOnce()
    wrapper.unmount()
  })
  it('pauses offscreen metrics, fences late replies and resumes when visible', async () => {
    let finish!: (value: unknown) => void
    const { state, wrapper, tab } = mountWorkspace()
    await flushPromises()
    api.metrics.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    await vi.advanceTimersByTimeAsync(5000)
    const signal = api.metrics.mock.calls.at(-1)![0] as AbortSignal
    tab.value = 'templates'
    await flushPromises()
    expect(signal.aborted).toBe(true)
    finish({ nodes: [{ nodeId: 'obsolete' }], fetchedAt: 'obsolete' })
    await flushPromises()
    expect(state.metrics.value?.fetchedAt).toBe('current')
    await vi.advanceTimersByTimeAsync(60000)
    expect(api.metrics).toHaveBeenCalledTimes(2)
    tab.value = 'metrics'
    await flushPromises()
    expect(api.metrics).toHaveBeenCalledTimes(3)
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(true)
    document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(60000)
    expect(api.metrics).toHaveBeenCalledTimes(3)
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
    expect(api.metrics).toHaveBeenCalledTimes(4)
    window.dispatchEvent(new Event('txc:session-cleared'))
    expect(state.metrics.value).toBeNull()
    await vi.advanceTimersByTimeAsync(60000)
    expect(api.metrics).toHaveBeenCalledTimes(4)
    wrapper.unmount()
  })
  it('marks prior observations stale and backs off repeated telemetry failures', async () => {
    api.metrics.mockResolvedValueOnce({ nodes: [{ nodeId: 'node', stale: false }], fetchedAt: 'current' }).mockRejectedValue(new Error('offline'))
    const { state, wrapper } = mountWorkspace()
    await flushPromises()
    await vi.advanceTimersByTimeAsync(5000)
    expect(state.metrics.value?.nodes[0]?.stale).toBe(true)
    expect(api.metrics).toHaveBeenCalledTimes(2)
    await vi.advanceTimersByTimeAsync(9999)
    expect(api.metrics).toHaveBeenCalledTimes(2)
    await vi.advanceTimersByTimeAsync(1)
    expect(api.metrics).toHaveBeenCalledTimes(3)
    await vi.advanceTimersByTimeAsync(19999)
    expect(api.metrics).toHaveBeenCalledTimes(3)
    wrapper.unmount()
  })

})
