import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cachedTransport } from './transport'
import { responseCacheKey } from './policy'
import { readResponseCache, setCacheSession } from './session'

const path = '/api/v1/dashboard'
const key = responseCacheKey(path)!
function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>(done => { resolve = done })
  return { promise, resolve }
}
beforeEach(() => { setCacheSession(null); setCacheSession('member:one') })
afterEach(() => setCacheSession(null))

describe('response cache transport', () => {
  it('retains dashboard display data without retaining its legacy bearer URL', async () => {
    const live = { balance: 10, subscriptionUrl: 'https://example.test/private-token' }
    expect(await cachedTransport(path, {}, async () => live)).toEqual(live)
    expect(readResponseCache(key)).toEqual({ balance: 10, subscriptionUrl: null })
    expect(live.subscriptionUrl).toBe('https://example.test/private-token')
  })
  it('always awaits a new backend response while the snapshot stays available', async () => {
    await cachedTransport(path, {}, async () => ({ balance: 10 }))
    const next = deferred<{ balance: number }>()
    const fresh = cachedTransport(path, {}, () => next.promise)
    expect(readResponseCache(key)).toEqual({ balance: 10, subscriptionUrl: null })
    next.resolve({ balance: 20 })
    expect(await fresh).toEqual({ balance: 20 })
    expect(readResponseCache(key)).toEqual({ balance: 20, subscriptionUrl: null })
  })

  it('shares overlapping reads but keeps sequential refreshes live', async () => {
    const next = deferred<number>()
    const send = vi.fn(() => next.promise)
    const first = cachedTransport(path, {}, send)
    const second = cachedTransport(path, {}, send)
    expect(send).toHaveBeenCalledTimes(1)
    next.resolve(1)
    await Promise.all([first, second])
    await cachedTransport(path, {}, send)
    expect(send).toHaveBeenCalledTimes(2)
  })

  it('neither retains nor shares explicit no-store responses', async () => {
    const first = deferred<number>()
    const second = deferred<number>()
    const send = vi.fn().mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise)
    const firstRead = cachedTransport(path, { cache: 'no-store' }, send)
    const secondRead = cachedTransport(path, { cache: 'no-store' }, send)
    expect(send).toHaveBeenCalledTimes(2)
    first.resolve(1)
    second.resolve(2)
    expect(await Promise.all([firstRead, secondRead])).toEqual([1, 2])
    expect(readResponseCache(key)).toBeUndefined()
  })

  it('does not overwrite a newer snapshot with a slower previous read', async () => {
    const old = deferred<number>()
    const request = cachedTransport(path, { signal: new AbortController().signal }, () => old.promise)
    await cachedTransport(path, {}, async () => 2)
    old.resolve(1)
    await request
    expect(readResponseCache(key)).toBe(2)
  })

  it('invalidates on mutation and rejects already running reads', async () => {
    const old = deferred<number>()
    const read = cachedTransport(path, {}, () => old.promise)
    await cachedTransport('/api/v1/purchases', { method: 'POST' }, async () => ({ id: 'purchase' }))
    old.resolve(1)
    await read
    expect(readResponseCache(key)).toBeUndefined()
  })

  it('keeps rendering snapshots through read-only DIY previews without caching preview contents', async () => {
    await cachedTransport(path, {}, async () => ({ balance: 10, subscriptionUrl: 'private' }))
    for (const previewPath of ['/api/v1/xnode/diy/preview', '/api/v1/xnode/routing/preview']) {
      await cachedTransport(previewPath, { method: 'POST', cache: 'no-store' }, async () => ({ valid: true }))
      expect(readResponseCache(key)).toEqual({ balance: 10, subscriptionUrl: null })
      expect(responseCacheKey(previewPath, { method: 'POST', cache: 'no-store' })).toBeNull()
    }
  })

  it('keeps ordinary errors visible without saving failed data', async () => {
    await cachedTransport(path, {}, async () => 1)
    const error = { status: 503 }
    await expect(cachedTransport(path, {}, async () => { throw error })).rejects.toBe(error)
    expect(readResponseCache(key)).toBe(1)
    await expect(cachedTransport(path, {}, async () => { throw { status: 401 } })).rejects.toEqual({ status: 401 })
    expect(readResponseCache(key)).toBeUndefined()
  })

  it('separates query/body variants and excludes authorization/action responses', () => {
    expect(responseCacheKey('/api/v1/admin/users?search=one&page=2')).toBe(responseCacheKey('/api/v1/admin/users?page=2&search=one'))
    expect(responseCacheKey('/api/v1/admin/users?page=1')).not.toBe(responseCacheKey('/api/v1/admin/users?page=2'))
    expect(responseCacheKey('/api/v1/community/membership/check')).not.toBe(responseCacheKey('/api/v1/community/membership/check', { method: 'POST' }))
    expect(responseCacheKey('/api/v1/admin/database/tables/users/query', { method: 'POST', body: { filters: [] } })).not.toBeNull()
    for (const excluded of ['/api/v1/me', '/api/v1/operations/one', '/api/v1/payments/orders/one', '/api/v1/admin/backups/one/key']) {
      expect(responseCacheKey(excluded)).toBeNull()
    }
    expect(responseCacheKey('/api/v1/purchases/quote', { method: 'POST' })).toBeNull()
  })
})
