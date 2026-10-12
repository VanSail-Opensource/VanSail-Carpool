import { defineComponent, reactive, nextTick } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { IPLookupProxySourcesStatus } from '@/api/ipLookup'

const mocks = vi.hoisted(() => ({ sourcesStatus: vi.fn(), testSource: vi.fn() }))
vi.mock('@/api/ipLookup', () => ({ ipLookupApi: mocks }))
const account = reactive({ user: { id: 'admin' } })
vi.mock('@/stores/session', () => ({ useSessionStore: () => account }))
import { useProxySources } from './useProxySources'

let state: ReturnType<typeof useProxySources>
const Harness = defineComponent({ setup() { state = useProxySources(); return () => null } })
const status = (refreshing = false): IPLookupProxySourcesStatus => ({ uniqueIPs: 2, sources: ['one', 'two'].map(id => ({ id, name: id, status: refreshing ? 'processing' : 'success', refreshing, enabled: true, stale: false, count: 2, observedAt: '', expiresAt: '', nextRefreshAt: '' })) })

describe('shared proxy source polling', () => {
  beforeEach(() => { vi.resetAllMocks(); vi.spyOn(document, 'hidden', 'get').mockReturnValue(false); account.user.id = 'admin' })
  afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks() })

  it('loads all rows through one request and refreshes only the selected source', async () => {
    mocks.sourcesStatus.mockResolvedValue(status())
    mocks.testSource.mockResolvedValue({ id: 'refresh' })
    const wrapper = mount(Harness)
    await flushPromises()
    expect(mocks.sourcesStatus).toHaveBeenCalledTimes(1)
    expect(state.byId.value.size).toBe(2)
    await state.refreshSource('one')
    expect(mocks.testSource).toHaveBeenCalledExactlyOnceWith('one')
    expect(state.status.value?.uniqueIPs).toBe(2)
    wrapper.unmount()
  })

  it('joins startup refreshes and retains status on refresh failure', async () => {
    mocks.sourcesStatus.mockResolvedValue(status(true))
    const wrapper = mount(Harness)
    await flushPromises()
    await state.refreshSource('one')
    expect(mocks.testSource).not.toHaveBeenCalled()
    mocks.sourcesStatus.mockResolvedValue(status())
    await state.load()
    mocks.testSource.mockRejectedValue(new Error('refresh unavailable'))
    await state.refreshSource('one')
    expect(state.errorCode.value).toBe('IP_LOOKUP_FAILED')
    expect(state.status.value?.uniqueIPs).toBe(2)
    wrapper.unmount()
  })

  it('rejects late account results and aborts reads on disposal', async () => {
    const pending: Array<(value: IPLookupProxySourcesStatus) => void> = []
    mocks.sourcesStatus.mockImplementation(() => new Promise(resolve => pending.push(resolve)))
    const wrapper = mount(Harness)
    const firstSignal = mocks.sourcesStatus.mock.calls[0]![0] as AbortSignal
    account.user.id = 'next-admin'
    await nextTick()
    expect(firstSignal.aborted).toBe(true)
    pending[0]!(status())
    await flushPromises()
    expect(state.status.value).toBeNull()
    pending[1]!(status())
    await flushPromises()
    expect(state.status.value?.uniqueIPs).toBe(2)
    const currentSignal = mocks.sourcesStatus.mock.calls[1]![0] as AbortSignal
    wrapper.unmount()
    expect(currentSignal.aborted).toBe(true)
  })

  it('retains a single polling timer after refresh and visibility changes', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    mocks.sourcesStatus.mockResolvedValue(status())
    mocks.testSource.mockResolvedValue({ id: 'refresh' })
    const wrapper = mount(Harness)
    await flushPromises()
    await state.refreshSource('one')
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
    const calls = mocks.sourcesStatus.mock.calls.length
    await vi.advanceTimersByTimeAsync(15000)
    expect(mocks.sourcesStatus).toHaveBeenCalledTimes(calls + 1)
    expect(vi.getTimerCount()).toBe(1)
    wrapper.unmount()
    expect(vi.getTimerCount()).toBe(0)
  })
})
