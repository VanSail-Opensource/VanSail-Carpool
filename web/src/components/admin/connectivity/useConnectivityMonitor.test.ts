import { mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { connectivityApi, type ConnectivitySnapshot } from '@/api/connectivity'
import { defaultConnectivityConfig } from './config'
import { useConnectivityMonitor } from './useConnectivityMonitor'

vi.mock('@/api/connectivity', () => ({ connectivityApi: { snapshot: vi.fn(), check: vi.fn() } }))
afterEach(() => { vi.clearAllMocks(); vi.useRealTimers() })

describe('connectivity polling lifecycle', () => {
  it('ignores superseded responses and cancels all polling after disposal', async () => {
    vi.useFakeTimers()
    const responses: Array<(value: ConnectivitySnapshot) => void> = []
    const signals: AbortSignal[] = []
    vi.mocked(connectivityApi.snapshot).mockImplementation(signal => {
      signals.push(signal!)
      return new Promise(resolve => { responses.push(resolve) })
    })
    let monitor!: ReturnType<typeof useConnectivityMonitor>
    const wrapper = mount(defineComponent({ setup() { monitor = useConnectivityMonitor(); return () => null } }))
    const refresh = monitor.refresh()
    const latest: ConnectivitySnapshot = { config: { ...defaultConnectivityConfig, proxyUserId: 42 }, user: null, run: null, hosts: [], stale: false, errorCode: '' }
    responses[1]!(latest)
    await refresh
    responses[0]!({ ...latest, config: { ...latest.config, proxyUserId: 41 } })
    await Promise.resolve()
    expect(signals[0]?.aborted).toBe(true)
    expect(monitor.snapshot.value?.config.proxyUserId).toBe(42)
    wrapper.unmount()
    await vi.advanceTimersByTimeAsync(60000)
    expect(connectivityApi.snapshot).toHaveBeenCalledTimes(2)
    expect(signals[1]?.aborted).toBe(true)
  })
})
