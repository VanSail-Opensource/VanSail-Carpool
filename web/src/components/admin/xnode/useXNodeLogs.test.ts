import { defineComponent, nextTick, shallowRef } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
const api = vi.hoisted(() => ({ logs: vi.fn() }))
vi.mock('@/api/xnode', () => ({ xnodeApi: api }))
import { useXNodeLogs } from './useXNodeLogs'

describe('node log request lifetime', () => {
  beforeEach(() => vi.resetAllMocks())
  it('discards stale node replies and clears logs on close or session expiry', async () => {
    let resolve!: (value: unknown) => void
    api.logs.mockReturnValueOnce(new Promise(done => { resolve = done })).mockResolvedValue({ nodeId: 'second', entries: [{ text: 'second node' }], dropped: '0' })
    const id = shallowRef('first')
    let state!: ReturnType<typeof useXNodeLogs>
    const wrapper = mount(defineComponent({ setup() { state = useXNodeLogs(() => id.value); return () => null } }))
    const firstSignal = api.logs.mock.calls[0]![1] as AbortSignal
    id.value = 'second'; await nextTick(); await flushPromises()
    resolve({ nodeId: 'first', entries: [{ text: 'private stale log' }] }); await flushPromises()
    expect(firstSignal.aborted).toBe(true)
    expect(state.text.value).toBe('second node')
    window.dispatchEvent(new Event('txc:session-cleared')); await nextTick()
    expect(state.snapshot.value).toBeNull()
    expect(state.paused.value).toBe(true)
    id.value = ''; await nextTick(); expect(state.text.value).toBe('')
    wrapper.unmount()
  })
})
