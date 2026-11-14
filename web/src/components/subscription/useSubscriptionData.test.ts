import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { setCacheSession } from '@/api/cache/session'
import { useSubscriptionData } from './useSubscriptionData'

afterEach(() => { setCacheSession(null); vi.useRealTimers() })

function mountController<T>(fetcher: (signal: AbortSignal) => Promise<T>) {
  let controls!: ReturnType<typeof useSubscriptionData<T>>
  const wrapper = mount(defineComponent({ setup() {
    controls = useSubscriptionData(fetcher)
    return () => null
  } }))
  return { wrapper, controls }
}

describe('live subscription lifecycle', () => {
  it('clears revoked keys, ignores obsolete responses and disposes private state', async () => {
    type Value = { key: string }
    const calls: { signal: AbortSignal; resolve: (value: Value) => void }[] = []
    const { wrapper, controls } = mountController(signal => new Promise<Value>(resolve => calls.push({ signal, resolve })))
    calls[0]!.resolve({ key: 'previous-owner-key' })
    await flushPromises()
    expect(controls.value.value?.key).toBe('previous-owner-key')
    const obsolete = controls.refresh()
    const current = controls.refresh(true)
    expect(calls[1]!.signal.aborted).toBe(true)
    expect(controls.value.value).toBeNull()
    calls[2]!.resolve({ key: 'rotated-owner-key' })
    await current
    calls[1]!.resolve({ key: 'obsolete-key' })
    await obsolete
    expect(controls.value.value?.key).toBe('rotated-owner-key')
    wrapper.unmount()
    expect(controls.value.value).toBeNull()
    expect(calls[2]!.signal.aborted).toBe(true)
  })

  it('polls only visible pages and resumes with a fresh read', async () => {
    vi.useFakeTimers()
    const fetcher = vi.fn(async () => ({ activeCombo: true }))
    Object.defineProperty(document, 'hidden', { configurable: true, value: false })
    const { wrapper } = mountController(fetcher)
    await flushPromises()
    expect(fetcher).toHaveBeenCalledTimes(1)
    Object.defineProperty(document, 'hidden', { configurable: true, value: true })
    document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(120_000)
    expect(fetcher).toHaveBeenCalledTimes(1)
    Object.defineProperty(document, 'hidden', { configurable: true, value: false })
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
    expect(fetcher).toHaveBeenCalledTimes(2)
    wrapper.unmount()
    await vi.advanceTimersByTimeAsync(120_000)
    expect(fetcher).toHaveBeenCalledTimes(2)
  })

  it.each(['success', 'failure'] as const)('retires credentials and ignores a late %s while a surviving view reloads its identity', async outcome => {
    vi.useFakeTimers()
    setCacheSession('member:one')
    Object.defineProperty(document, 'hidden', { configurable: true, value: false })
    type Value = { key: string }
    const calls: { signal: AbortSignal; resolve: (value: Value) => void; reject: (error: Error) => void }[] = []
    const { wrapper, controls } = mountController(signal => new Promise<Value>((resolve, reject) => calls.push({ signal, resolve, reject })))
    calls[0]!.resolve({ key: 'previous-identity-key' })
    await flushPromises()
    expect(controls.liveReady.value).toBe(true)
    const obsolete = controls.refresh()
    setCacheSession('member:one:admin')
    expect(controls.value.value).toBeNull()
    expect(controls.liveReady.value).toBe(false)
    expect(controls.error.value).toBeNull()
    expect(calls[1]!.signal.aborted).toBe(true)
    await flushPromises()
    expect(calls).toHaveLength(3)
    if (outcome === 'success') calls[1]!.resolve({ key: 'obsolete-key' })
    else calls[1]!.reject(new Error('old identity failure'))
    await obsolete
    expect(controls.value.value).toBeNull()
    expect(controls.liveReady.value).toBe(false)
    expect(controls.error.value).toBeNull()
    expect(controls.refreshing.value).toBe(true)
    calls[2]!.resolve({ key: 'current-identity-key' })
    await flushPromises()
    expect(controls.value.value?.key).toBe('current-identity-key')
    expect(controls.liveReady.value).toBe(true)
    setCacheSession(null)
    expect(controls.value.value).toBeNull()
    expect(controls.liveReady.value).toBe(false)
    expect(calls[2]!.signal.aborted).toBe(true)
    await flushPromises()
    window.dispatchEvent(new Event('focus'))
    await controls.refresh()
    await vi.advanceTimersByTimeAsync(120_000)
    expect(calls).toHaveLength(3)
    wrapper.unmount()
  })
})
