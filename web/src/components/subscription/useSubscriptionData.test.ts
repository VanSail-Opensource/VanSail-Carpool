import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { setCacheSession } from '@/api/cache/session'
import { useSubscriptionData } from './useSubscriptionData'

const hiddenDescriptor = Object.getOwnPropertyDescriptor(document, 'hidden')
beforeEach(() => { Object.defineProperty(document, 'hidden', { configurable: true, value: false }) })
afterEach(() => {
  setCacheSession(null); vi.useRealTimers()
  if (hiddenDescriptor) Object.defineProperty(document, 'hidden', hiddenDescriptor)
  else Reflect.deleteProperty(document, 'hidden')
})

function mountController<T>(fetcher: (signal: AbortSignal) => Promise<T>, interval = 60_000) {
  let controls!: ReturnType<typeof useSubscriptionData<T>>
  const wrapper = mount(defineComponent({ setup() {
    controls = useSubscriptionData(fetcher, interval)
    return () => null
  } }))
  return { wrapper, controls }
}

describe('live subscription lifecycle', () => {
  it('waits for visibility before its initial read and coalesces resume focus events', async () => {
    vi.useFakeTimers()
    Object.defineProperty(document, 'hidden', { configurable: true, value: true })
    let finish!: (value: string) => void
    const fetcher = vi.fn(() => new Promise<string>(resolve => { finish = resolve }))
    const { wrapper, controls } = mountController(fetcher, 5000)
    await controls.refresh()
    await vi.advanceTimersByTimeAsync(60_000)
    expect(fetcher).not.toHaveBeenCalled()
    Object.defineProperty(document, 'hidden', { configurable: true, value: false })
    document.dispatchEvent(new Event('visibilitychange'))
    window.dispatchEvent(new Event('focus'))
    window.dispatchEvent(new Event('focus'))
    expect(fetcher).toHaveBeenCalledOnce()
    expect(controls.refreshing.value).toBe(true)
    finish('current')
    await flushPromises()
    expect(controls.value.value).toBe('current')
    wrapper.unmount()
  })

  it('keeps forced clearing effective while hidden and retires the old in-flight read', async () => {
    const calls: { signal: AbortSignal; resolve: (value: string) => void }[] = []
    const { wrapper, controls } = mountController(signal => new Promise<string>(resolve => calls.push({ signal, resolve })))
    calls[0]!.resolve('revoked-key')
    await flushPromises()
    const obsolete = controls.refresh()
    Object.defineProperty(document, 'hidden', { configurable: true, value: true })
    await controls.refresh(true)
    expect(controls.value.value).toBeNull()
    expect(controls.liveReady.value).toBe(false)
    expect(controls.refreshing.value).toBe(false)
    expect(calls).toHaveLength(2)
    expect(calls[1]!.signal.aborted).toBe(true)
    calls[1]!.resolve('obsolete-key')
    await obsolete
    expect(controls.value.value).toBeNull()
    Object.defineProperty(document, 'hidden', { configurable: true, value: false })
    document.dispatchEvent(new Event('visibilitychange'))
    expect(calls).toHaveLength(3)
    calls[2]!.resolve('rotated-key')
    await flushPromises()
    expect(controls.value.value).toBe('rotated-key')
    wrapper.unmount()
  })

  it('waits five seconds after completion and never overlaps a slow periodic read', async () => {
    vi.useFakeTimers()
    const calls: { signal: AbortSignal; resolve: (value: string) => void }[] = []
    const { wrapper } = mountController(signal => new Promise<string>(resolve => calls.push({ signal, resolve })), 5000)
    await vi.advanceTimersByTimeAsync(15_000)
    expect(calls).toHaveLength(1)
    expect(calls[0]!.signal.aborted).toBe(false)
    calls[0]!.resolve('first')
    await flushPromises()
    await vi.advanceTimersByTimeAsync(4999)
    expect(calls).toHaveLength(1)
    await vi.advanceTimersByTimeAsync(1)
    expect(calls).toHaveLength(2)
    await vi.advanceTimersByTimeAsync(15_000)
    expect(calls).toHaveLength(2)
    expect(calls[1]!.signal.aborted).toBe(false)
    wrapper.unmount()
  })

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
