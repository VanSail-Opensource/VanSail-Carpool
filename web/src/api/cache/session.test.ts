import { watch } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cacheGeneration, cacheIdentityGeneration, clearResponseCache, readResponseCache, setCacheSession, writeResponseCache } from './session'

beforeEach(() => { setCacheSession(null); setCacheSession('member:one') })
afterEach(() => { setCacheSession(null); vi.useRealTimers() })

describe('session response snapshots', () => {
  it('expires short-lived displays without extending freshness on cache hits', () => {
    vi.useFakeTimers()
    writeResponseCache('hourly', { price: '100' }, cacheGeneration())
    vi.advanceTimersByTime(59_000)
    expect(readResponseCache('hourly', 60_000)).toEqual({ price: '100' })
    vi.advanceTimersByTime(1001)
    expect(readResponseCache('hourly', 60_000)).toBeUndefined()
  })
  it('isolates saved data from both callers and editable restored drafts', () => {
    const response = { items: [{ name: 'Singapore' }] }
    writeResponseCache('catalog', response, cacheGeneration())
    response.items[0]!.name = 'Changed outside cache'
    const first = readResponseCache<typeof response>('catalog')!
    first.items[0]!.name = 'Unsaved edit'
    expect(readResponseCache('catalog')).toEqual({ items: [{ name: 'Singapore' }] })
  })

  it('retains empty and null responses without treating them as misses', () => {
    writeResponseCache('empty-resource', null, cacheGeneration())
    writeResponseCache('records', { items: [] }, cacheGeneration())
    expect(readResponseCache('empty-resource')).toBeNull()
    expect(readResponseCache('records')).toEqual({ items: [] })
    expect(readResponseCache('missing')).toBeUndefined()
  })

  it('discards data and late responses across account, role and logout changes', () => {
    const old = cacheGeneration()
    writeResponseCache('dashboard', { account: 'one' }, old)
    setCacheSession('member:two')
    writeResponseCache('dashboard', { account: 'one' }, old)
    expect(readResponseCache('dashboard')).toBeUndefined()
    writeResponseCache('dashboard', { account: 'two' }, cacheGeneration())
    setCacheSession('admin:two')
    expect(readResponseCache('dashboard')).toBeUndefined()
    setCacheSession(null)
    writeResponseCache('dashboard', { account: 'two' }, cacheGeneration())
    expect(readResponseCache('dashboard')).toBeUndefined()
  })

  it('rejects in-flight responses from before mutation invalidation', () => {
    const started = cacheGeneration()
    clearResponseCache()
    writeResponseCache('balance', { minor: '1000' }, started)
    expect(readResponseCache('balance')).toBeUndefined()
  })

  it('clears the old snapshots before synchronous identity observers can read them', () => {
    writeResponseCache('dashboard', { account: 'one' }, cacheGeneration())
    const observed: unknown[] = []
    const stop = watch(cacheIdentityGeneration, () => {
      observed.push(readResponseCache('dashboard'))
    }, { flush: 'sync' })
    setCacheSession('member:two')
    stop()
    expect(observed).toEqual([undefined])
  })

  it('bounds retained entries while keeping recently visited pages', () => {
    for (let index = 0; index < 256; index++) writeResponseCache(String(index), index, cacheGeneration())
    expect(readResponseCache('0')).toBe(0)
    writeResponseCache('new', 'new', cacheGeneration())
    expect(readResponseCache('0')).toBe(0)
    expect(readResponseCache('1')).toBeUndefined()
    writeResponseCache('oversize', 'x'.repeat(3 * 1024 * 1024), cacheGeneration())
    expect(readResponseCache('oversize')).toBeUndefined()
  })
})
