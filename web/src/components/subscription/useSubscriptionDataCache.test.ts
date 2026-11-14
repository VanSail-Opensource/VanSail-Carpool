import { defineComponent } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { PAYGView } from '@/api/payg'
import { responseCacheKey } from '@/api/cache/policy'
import { cacheGeneration, setCacheSession, writeResponseCache } from '@/api/cache/session'
import { useSubscriptionData } from './useSubscriptionData'

const path = '/api/v1/payg'
const key = responseCacheKey(path)!
const mounted: VueWrapper[] = []
const hiddenDescriptor = Object.getOwnPropertyDescriptor(document, 'hidden')
function snapshot(minor: string): PAYGView {
  return {
    balance: { currency: 'TXB', minor, display: `${(Number(minor) / 100).toFixed(2)} TXB` },
    windowStart: '2026-11-14T00:00:00Z', asOf: '2026-11-14T01:00:00Z',
    trafficBytes: '0', spent: { currency: 'TXB', minor: '0', display: '0 TXB' }, squads: [],
  }
}
function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (error: Error) => void
  const promise = new Promise<T>((done, fail) => { resolve = done; reject = fail })
  return { promise, resolve, reject }
}
function controller(fetcher: (signal: AbortSignal) => Promise<PAYGView>, cachePath?: string) {
  let state!: ReturnType<typeof useSubscriptionData<PAYGView>>
  const wrapper = mount(defineComponent({ setup() {
    state = useSubscriptionData(fetcher, 60_000, 'payg.loadFailed', undefined, cachePath)
    return () => null
  } }))
  mounted.push(wrapper)
  return { state, wrapper }
}

beforeEach(() => {
  vi.useFakeTimers()
  setCacheSession(null); setCacheSession('member:one')
  Object.defineProperty(document, 'hidden', { configurable: true, value: false })
})
afterEach(() => {
  mounted.splice(0).forEach(wrapper => wrapper.unmount())
  setCacheSession(null)
  vi.restoreAllMocks(); vi.useRealTimers()
  if (hiddenDescriptor) Object.defineProperty(document, 'hidden', hiddenDescriptor)
  else Reflect.deleteProperty(document, 'hidden')
})

describe('optional member display snapshots', () => {
  it('renders an existing snapshot immediately but authorizes actions only after fresh success', async () => {
    const cached = snapshot('100'), fresh = snapshot('250'), read = deferred<PAYGView>()
    writeResponseCache(key, cached, cacheGeneration())
    const { state } = controller(() => read.promise, path)
    expect(state.value.value).toEqual(cached)
    expect(state.hasData.value).toBe(true)
    expect(state.loading.value).toBe(false)
    expect(state.refreshing.value).toBe(true)
    expect(state.liveReady.value).toBe(false)
    read.resolve(fresh)
    await flushPromises()
    expect(state.value.value).toEqual(fresh)
    expect(state.refreshing.value).toBe(false)
    expect(state.liveReady.value).toBe(true)
  })

  it('does not restore an expired snapshot or implicitly opt a live-only loader into caching', () => {
    writeResponseCache(key, snapshot('100'), cacheGeneration())
    const firstRead = deferred<PAYGView>(), secondRead = deferred<PAYGView>()
    const freshOnly = controller(() => firstRead.promise)
    expect(freshOnly.state.value.value).toBeNull()
    expect(freshOnly.state.loading.value).toBe(true)
    vi.advanceTimersByTime(60_001)
    const expired = controller(() => secondRead.promise, path)
    expect(expired.state.value.value).toBeNull()
    expect(expired.state.loading.value).toBe(true)
    expect(expired.state.liveReady.value).toBe(false)
  })

  it('keeps a cached display on read failure and waits for successful retry before live readiness', async () => {
    const cached = snapshot('100'), fresh = snapshot('250')
    writeResponseCache(key, cached, cacheGeneration())
    const fetcher = vi.fn().mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce(fresh)
    const { state } = controller(fetcher, path)
    await flushPromises()
    expect(state.value.value).toEqual(cached)
    expect(state.loading.value).toBe(false)
    expect(state.liveReady.value).toBe(false)
    expect(state.error.value).toBeTruthy()
    await state.refresh()
    expect(state.value.value).toEqual(fresh)
    expect(state.liveReady.value).toBe(true)
    expect(state.error.value).toBeNull()
  })

  it.each(['success', 'failure'] as const)('ignores a late %s after identity replacement without restarting the old poller', async outcome => {
    const old = deferred<PAYGView>(), oldFetcher = vi.fn(() => old.promise)
    const obsolete = controller(oldFetcher, path)
    setCacheSession('member:two')
    const current = controller(async () => snapshot('900'), path)
    await flushPromises()
    expect(current.state.value.value?.balance.minor).toBe('900')
    expect(current.state.liveReady.value).toBe(true)
    if (outcome === 'success') old.resolve(snapshot('100'))
    else old.reject(new Error('old account read failed'))
    await flushPromises()
    expect(obsolete.state.value.value).toBeNull()
    expect(obsolete.state.error.value).toBeNull()
    expect(obsolete.state.liveReady.value).toBe(false)
    expect(current.state.value.value?.balance.minor).toBe('900')
    await vi.advanceTimersByTimeAsync(60_001)
    expect(oldFetcher).toHaveBeenCalledTimes(1)
  })
})
